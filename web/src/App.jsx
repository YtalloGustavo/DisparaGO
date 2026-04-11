import { useEffect, useRef, useState } from "react";
import { LayoutDashboard, Settings, LogOut } from "lucide-react";

const POLL_INTERVAL_MS = 8000;
const AUTH_STORAGE_KEY = "disparago.auth";
const FINAL_STATUSES = new Set(["sent", "delivered", "read", "failed", "partial"]);

async function request(url, options = {}, token) {
  const headers = new Headers(options.headers || {});
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }

  const response = await fetch(url, {
    ...options,
    headers,
  });

  const text = await response.text();
  const payload = text ? JSON.parse(text) : {};

  if (!response.ok) {
    const error = new Error(payload.error || "Falha na requisicao");
    error.status = response.status;
    throw error;
  }

  return payload;
}

function formatDate(value) {
  if (!value) return "-";
  return new Date(value).toLocaleString("pt-BR");
}

function statusClass(status) {
  return (status || "pending").toLowerCase();
}

function aggregateStats(campaigns) {
  const active = campaigns.filter((item) =>
    ["pending", "processing", "paused"].includes(statusClass(item.status))
  ).length;

  return {
    total: campaigns.length,
    active,
    sent: campaigns.reduce((sum, item) => sum + (item.sent_count || 0), 0),
    failed: campaigns.reduce((sum, item) => sum + (item.failed_count || 0), 0),
  };
}

function normalizeSession(data) {
  if (!data?.token) return null;

  return {
    token: data.token,
    username: data.username || "admin",
    role: data.role || "operator",
    expiresAt: data.expiresAt || data.expires_at || "",
  };
}

function saveSession(session) {
  window.localStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify(session));
}

function loadStoredSession() {
  const raw = window.localStorage.getItem(AUTH_STORAGE_KEY);
  if (!raw) return null;

  try {
    return normalizeSession(JSON.parse(raw));
  } catch {
    window.localStorage.removeItem(AUTH_STORAGE_KEY);
    return null;
  }
}

function clearStoredSession() {
  window.localStorage.removeItem(AUTH_STORAGE_KEY);
}

function buildSettingsForm(data) {
  return {
    humanizer_enabled: Boolean(data?.humanizer_enabled),
    initial_delay_min_seconds: String(data?.initial_delay_min_seconds ?? 3),
    initial_delay_max_seconds: String(data?.initial_delay_max_seconds ?? 8),
    base_delay_min_seconds: String(data?.base_delay_min_seconds ?? 9),
    base_delay_max_seconds: String(data?.base_delay_max_seconds ?? 18),
    provider_delay_min_ms: String(data?.provider_delay_min_ms ?? 1500),
    provider_delay_max_ms: String(data?.provider_delay_max_ms ?? 5000),
    burst_size_min: String(data?.burst_size_min ?? 4),
    burst_size_max: String(data?.burst_size_max ?? 8),
    burst_pause_min_seconds: String(data?.burst_pause_min_seconds ?? 45),
    burst_pause_max_seconds: String(data?.burst_pause_max_seconds ?? 120),
    webhook_enabled: Boolean(data?.webhook_enabled),
    webhook_subscriptions: (data?.webhook_subscriptions || ["ALL"]).join(", "),
  };
}

function detectPreset(form) {
  const signature = [
    form.initial_delay_min_seconds,
    form.initial_delay_max_seconds,
    form.base_delay_min_seconds,
    form.base_delay_max_seconds,
    form.provider_delay_min_ms,
    form.provider_delay_max_ms,
    form.burst_size_min,
    form.burst_size_max,
    form.burst_pause_min_seconds,
    form.burst_pause_max_seconds,
  ].join("|");

  if (signature === "8|16|18|35|3000|7000|2|4|90|240") return "conservative";
  if (signature === "4|10|12|24|2000|6000|3|5|60|150") return "balanced";
  if (signature === "2|6|8|16|1200|3500|4|7|35|90") return "faster";
  return "custom";
}

function App() {
  const [session, setSession] = useState(null);
  const [checkingAuth, setCheckingAuth] = useState(true);
  const [authLoading, setAuthLoading] = useState(false);
  const [authError, setAuthError] = useState("");
  const [authForm, setAuthForm] = useState({
    username: "",
    password: "",
  });

  const [campaigns, setCampaigns] = useState([]);
  const [selectedCampaign, setSelectedCampaign] = useState(null);
  const [messages, setMessages] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [refreshStamp, setRefreshStamp] = useState("");
  const [actionLoading, setActionLoading] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [createSuccess, setCreateSuccess] = useState("");
  const [settingsLoading, setSettingsLoading] = useState(false);
  const [settingsSaving, setSettingsSaving] = useState(false);
  const [settingsSuccess, setSettingsSuccess] = useState("");
  const [settingsData, setSettingsData] = useState(null);
  const [settingsInstanceId, setSettingsInstanceId] = useState("");
  const [settingsForm, setSettingsForm] = useState(buildSettingsForm());
  const [activeView, setActiveView] = useState("operations");
  const [showAdvancedSettings, setShowAdvancedSettings] = useState(false);
  const [showWebhookToken, setShowWebhookToken] = useState(false);
  const [form, setForm] = useState({
    name: "",
    instanceId: "",
    message: "",
    contacts: "",
  });
  const selectedIdRef = useRef(null);

  const stats = aggregateStats(campaigns);

  useEffect(() => {
    const storedSession = loadStoredSession();
    if (!storedSession) {
      setCheckingAuth(false);
      setLoading(false);
      return;
    }

    request("/api/v1/auth/me", {}, storedSession.token)
      .then((response) => {
        const nextSession = {
          token: storedSession.token,
          username: response.data?.username || storedSession.username,
          role: response.data?.role || storedSession.role || "operator",
          expiresAt: response.data?.expires_at || storedSession.expiresAt,
        };
        saveSession(nextSession);
        setSession(nextSession);
      })
      .catch(() => {
        clearStoredSession();
      })
      .finally(() => {
        setCheckingAuth(false);
      });
  }, []);

  useEffect(() => {
    if (!session?.token) {
      setCampaigns([]);
      setSelectedCampaign(null);
      setMessages([]);
      return undefined;
    }

    refreshAll(session.token, true).catch(() => {});
    const timer = window.setInterval(() => {
      refreshAll(session.token, false).catch(() => {});
    }, POLL_INTERVAL_MS);

    return () => window.clearInterval(timer);
  }, [session?.token]);

  useEffect(() => {
    const nextInstanceId = selectedCampaign?.instance_id || "";
    if (!nextInstanceId || !session?.token || session?.role !== "superadmin") return;

    setSettingsInstanceId(nextInstanceId);
    loadInstanceSettings(nextInstanceId, session.token).catch(() => {});
  }, [selectedCampaign?.instance_id, session?.token]);

  useEffect(() => {
    if (session?.role !== "superadmin" && activeView === "settings") {
      setActiveView("operations");
    }
  }, [activeView, session?.role]);

  function applyLogout(nextMessage = "Sua sessao expirou. Entre novamente.") {
    clearStoredSession();
    setSession(null);
    setAuthForm((current) => ({ ...current, password: "" }));
    setAuthError(nextMessage);
    setError("");
    setLoading(false);
  }

  async function refreshAll(token, selectFirst) {
    try {
      if (loading) setLoading(true);
      const listResponse = await request("/api/v1/campaigns?limit=50", {}, token);
      const nextCampaigns = listResponse.data || [];
      setCampaigns(nextCampaigns);

      let nextSelectedId = selectedIdRef.current;
      if (!nextSelectedId && nextCampaigns.length && selectFirst) {
        nextSelectedId = nextCampaigns[0].id;
      }
      if (nextSelectedId && !nextCampaigns.some((item) => item.id === nextSelectedId)) {
        nextSelectedId = nextCampaigns[0]?.id || null;
      }

      if (nextSelectedId) {
        await loadCampaign(nextSelectedId, token);
      } else {
        setSelectedCampaign(null);
        setMessages([]);
      }

      setRefreshStamp(`Atualizado ${new Date().toLocaleTimeString("pt-BR")}`);
      setError("");
    } catch (err) {
      if (err.status === 401) {
        applyLogout();
        return;
      }
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }

  async function loadCampaign(campaignId, token = session?.token) {
    selectedIdRef.current = campaignId;

    try {
      const [campaignResponse, messagesResponse] = await Promise.all([
        request(`/api/v1/campaigns/${campaignId}`, {}, token),
        request(`/api/v1/campaigns/${campaignId}/messages`, {}, token),
      ]);

      setSelectedCampaign(campaignResponse.data);
      setMessages(messagesResponse.data.messages || []);
      setError("");
    } catch (err) {
      if (err.status === 401) {
        applyLogout();
        return;
      }
      setError(err.message);
    }
  }

  async function loadInstanceSettings(instanceId, token = session?.token) {
    if (!instanceId) return;

    try {
      setSettingsLoading(true);
      setSettingsSuccess("");
      const response = await request(`/api/v1/admin/instances/${instanceId}/settings`, {}, token);
      setSettingsData(response.data);
      setSettingsForm(buildSettingsForm(response.data));
      setError("");
    } catch (err) {
      if (err.status === 401) {
        applyLogout();
        return;
      }
      setError(err.message);
    } finally {
      setSettingsLoading(false);
    }
  }

  async function handleLogin(event) {
    event.preventDefault();

    try {
      setAuthLoading(true);
      setAuthError("");

      const response = await request("/api/v1/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(authForm),
      });

      const nextSession = normalizeSession({
        token: response.data?.token,
        username: response.data?.username,
        role: response.data?.role,
        expiresAt: response.data?.expires_at,
      });

      saveSession(nextSession);
      setSession(nextSession);
      setLoading(true);
      setAuthForm((current) => ({ ...current, password: "" }));
    } catch (err) {
      setAuthError(err.message);
    } finally {
      setAuthLoading(false);
    }
  }

  async function handleCreateCampaign(event) {
    event.preventDefault();

    try {
      setCreateLoading(true);
      setCreateSuccess("");
      setError("");

      const contacts = form.contacts
        .split(/\r?\n|,/)
        .map((item) => item.trim())
        .filter(Boolean);

      const payload = {
        name: form.name.trim(),
        instance_id: form.instanceId.trim(),
        message: form.message.trim(),
        contacts,
      };

      const created = await request(
        "/api/v1/campaigns",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        },
        session?.token
      );

      selectedIdRef.current = created.data.id;
      setForm({
        name: "",
        instanceId: payload.instance_id,
        message: "",
        contacts: "",
      });
      setCreateSuccess(
        `Campanha ${created.data.name} criada com ${created.data.total_messages} contatos.`
      );
      await refreshAll(session?.token, false);
    } catch (err) {
      if (err.status === 401) {
        applyLogout();
        return;
      }
      setError(err.message);
    } finally {
      setCreateLoading(false);
    }
  }

  async function handleCampaignAction(type) {
    if (!selectedCampaign) return;
    const endpoint = type === "pause" ? "pause" : "resume";

    try {
      setActionLoading(true);
      await request(
        `/api/v1/campaigns/${selectedCampaign.id}/${endpoint}`,
        { method: "POST" },
        session?.token
      );
      await refreshAll(session?.token, false);
    } catch (err) {
      if (err.status === 401) {
        applyLogout();
        return;
      }
      setError(err.message);
    } finally {
      setActionLoading(false);
    }
  }

  async function handleSaveInstanceSettings(event) {
    event.preventDefault();

    if (!settingsInstanceId.trim()) {
      setError("Informe a instancia para salvar a configuracao.");
      return;
    }

    try {
      setSettingsSaving(true);
      setSettingsSuccess("");
      setError("");

      const payload = {
        humanizer_enabled: settingsForm.humanizer_enabled,
        initial_delay_min_seconds: Number(settingsForm.initial_delay_min_seconds),
        initial_delay_max_seconds: Number(settingsForm.initial_delay_max_seconds),
        base_delay_min_seconds: Number(settingsForm.base_delay_min_seconds),
        base_delay_max_seconds: Number(settingsForm.base_delay_max_seconds),
        provider_delay_min_ms: Number(settingsForm.provider_delay_min_ms),
        provider_delay_max_ms: Number(settingsForm.provider_delay_max_ms),
        burst_size_min: Number(settingsForm.burst_size_min),
        burst_size_max: Number(settingsForm.burst_size_max),
        burst_pause_min_seconds: Number(settingsForm.burst_pause_min_seconds),
        burst_pause_max_seconds: Number(settingsForm.burst_pause_max_seconds),
        webhook_enabled: settingsForm.webhook_enabled,
        webhook_subscriptions: settingsForm.webhook_subscriptions
          .split(",")
          .map((item) => item.trim())
          .filter(Boolean),
      };

      const response = await request(
        `/api/v1/admin/instances/${settingsInstanceId.trim()}/settings`,
        {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        },
        session?.token
      );

      setSettingsData(response.data);
      setSettingsForm(buildSettingsForm(response.data));
      setSettingsSuccess(`Configuracao da instancia ${response.data.instance_id} salva.`);
    } catch (err) {
      if (err.status === 401) {
        applyLogout();
        return;
      }
      setError(err.message);
    } finally {
      setSettingsSaving(false);
    }
  }

  async function handleReloadSettings() {
    await loadInstanceSettings(settingsInstanceId);
  }

  async function handleCopyWebhook() {
    if (!settingsData?.webhook_url || !navigator?.clipboard) return;
    await navigator.clipboard.writeText(settingsData.webhook_url);
    setSettingsSuccess("URL do webhook copiada.");
  }

  function applyPreset(preset) {
    const presets = {
      conservative: {
        initial_delay_min_seconds: "8",
        initial_delay_max_seconds: "16",
        base_delay_min_seconds: "18",
        base_delay_max_seconds: "35",
        provider_delay_min_ms: "3000",
        provider_delay_max_ms: "7000",
        burst_size_min: "2",
        burst_size_max: "4",
        burst_pause_min_seconds: "90",
        burst_pause_max_seconds: "240",
      },
      balanced: {
        initial_delay_min_seconds: "4",
        initial_delay_max_seconds: "10",
        base_delay_min_seconds: "12",
        base_delay_max_seconds: "24",
        provider_delay_min_ms: "2000",
        provider_delay_max_ms: "6000",
        burst_size_min: "3",
        burst_size_max: "5",
        burst_pause_min_seconds: "60",
        burst_pause_max_seconds: "150",
      },
      faster: {
        initial_delay_min_seconds: "2",
        initial_delay_max_seconds: "6",
        base_delay_min_seconds: "8",
        base_delay_max_seconds: "16",
        provider_delay_min_ms: "1200",
        provider_delay_max_ms: "3500",
        burst_size_min: "4",
        burst_size_max: "7",
        burst_pause_min_seconds: "35",
        burst_pause_max_seconds: "90",
      },
    };

    const selected = presets[preset];
    if (!selected) return;

    setSettingsForm((current) => ({
      ...current,
      ...selected,
      humanizer_enabled: true,
    }));
    setSettingsSuccess("Perfil aplicado. Revise e clique em salvar configuracao.");
  }

  const canPause =
    selectedCampaign &&
    !selectedCampaign.paused &&
    !FINAL_STATUSES.has(statusClass(selectedCampaign.status));

  const canResume = selectedCampaign?.paused;
  const settingsPreset = detectPreset(settingsForm);
  const isSuperadmin = session?.role === "superadmin";

  if (checkingAuth) {
    return (
      <div className="auth-shell">
        <div className="auth-card">
          <div className="eyebrow">Disparador operacional</div>
          <h1>DisparaGO</h1>
          <p>Validando sessao do painel.</p>
        </div>
      </div>
    );
  }

  if (!session) {
    return (
      <div className="auth-shell">
        <section className="auth-card">
          <div className="eyebrow">Painel protegido</div>
          <h1>Entrar no DisparaGO</h1>
          <p>
            Acompanhe campanhas, tracking por mensagem e operacao em tempo real com acesso
            autenticado.
          </p>

          <form className="auth-form" onSubmit={handleLogin}>
            <label className="field">
              <span>Usuario</span>
              <input
                value={authForm.username}
                onChange={(event) =>
                  setAuthForm((current) => ({ ...current, username: event.target.value }))
                }
                placeholder="admin"
                autoComplete="username"
                required
              />
            </label>

            <label className="field">
              <span>Senha</span>
              <input
                type="password"
                value={authForm.password}
                onChange={(event) =>
                  setAuthForm((current) => ({ ...current, password: event.target.value }))
                }
                placeholder="Sua senha do painel"
                autoComplete="current-password"
                required
              />
            </label>

            <button className="primary-btn auth-submit" type="submit" disabled={authLoading}>
              {authLoading ? "Entrando..." : "Acessar painel"}
            </button>
          </form>

          {authError ? <div className="alert auth-alert">{authError}</div> : null}
        </section>
      </div>
    );
  }

  return (
    <div className="dashboard-layout">
      <aside className="sidebar">
        <div className="brand">
          Dispara<span>GO</span>
        </div>
        <nav className="nav-menu">
          <div className="nav-group">Atendimento & Operação</div>
          <button
            className={`nav-item ${activeView === "operations" ? "active" : ""}`}
            onClick={() => setActiveView("operations")}
          >
            <LayoutDashboard size={20} />
            <span>Operação Diária</span>
          </button>
          {isSuperadmin ? (
            <>
              <div className="nav-group">Gerência</div>
              <button
                className={`nav-item ${activeView === "settings" ? "active" : ""}`}
                onClick={() => setActiveView("settings")}
              >
                <Settings size={20} />
                <span>Configuração de Instância</span>
              </button>
            </>
          ) : null}
        </nav>
      </aside>

      <div className="main-area">
        <header className="topbar">
          <div className="topbar-welcome">Dashboard Operacional</div>
          <div className="topbar-actions">
            <div className="user-profile">
              <div className="user-info">
                <strong>{session.username}</strong>
                <span>Expira em {formatDate(session.expiresAt)}</span>
              </div>
            </div>
            <button className="logout-btn" onClick={() => applyLogout("Sessão encerrada.")}>
              <LogOut size={18} />
              <span>Sair</span>
            </button>
          </div>
        </header>

        <main className="content">
          <section className="hero">
            <div className="panel hero-card hero-main">
              <div className="eyebrow">Disparador operacional</div>
              <h1>DisparaGO</h1>
              <p>
                Um painel mais simples para criar campanhas, acompanhar entregas e ajustar o envio com segurança.
              </p>
            </div>

            <div className="panel hero-card side">
              <div className="stats-grid" style={{ marginTop: 0 }}>
                <StatCard label="Campanhas" value={stats.total} />
                <StatCard label="Ativas" value={stats.active} />
                <StatCard label="Enviadas" value={stats.sent} />
                <StatCard label="Falhas" value={stats.failed} />
              </div>
            </div>
          </section>

      {activeView === "operations" ? (
        <>
      <section className="panel create-panel">
        <div className="card-head">
          <div>
            <div className="card-title">Nova campanha</div>
            <div className="muted">Crie e dispare campanhas diretamente pelo painel.</div>
          </div>
        </div>
        <div className="card-body">
          <form className="create-form" onSubmit={handleCreateCampaign}>
            <label className="field">
              <span>Nome da campanha</span>
              <input
                value={form.name}
                onChange={(event) =>
                  setForm((current) => ({ ...current, name: event.target.value }))
                }
                placeholder="Ex.: Campanha Abril Premium"
                required
              />
            </label>

            <label className="field">
              <span>Instancia WhatsApp</span>
              <input
                value={form.instanceId}
                onChange={(event) =>
                  setForm((current) => ({ ...current, instanceId: event.target.value }))
                }
                placeholder="Ex.: servidoron_2_1"
                required
              />
            </label>

            <label className="field field-full">
              <span>Mensagem</span>
              <textarea
                value={form.message}
                onChange={(event) =>
                  setForm((current) => ({ ...current, message: event.target.value }))
                }
                placeholder="Digite a mensagem da campanha"
                rows={4}
                required
              />
            </label>

            <label className="field field-full">
              <span>Contatos</span>
              <textarea
                value={form.contacts}
                onChange={(event) =>
                  setForm((current) => ({ ...current, contacts: event.target.value }))
                }
                placeholder={"Um numero por linha\n5511999999999\n5511888888888"}
                rows={6}
                required
              />
            </label>

            <div className="form-footer">
              <div className="muted">Aceita um numero por linha ou separados por virgula.</div>
              <button className="primary-btn" type="submit" disabled={createLoading}>
                {createLoading ? "Criando..." : "Criar campanha"}
              </button>
            </div>
          </form>

          {createSuccess ? <div className="success-banner">{createSuccess}</div> : null}
        </div>
      </section>

        </>
      ) : null}

      {isSuperadmin && activeView === "settings" ? (
      <section className="panel settings-panel">
        <div className="card-head">
          <div>
            <div className="card-title">Configuracao da instancia</div>
            <div className="muted">
              Area de administracao para ajustar envio e conectar o webhook no provider.
            </div>
          </div>
        </div>
        <div className="card-body">
          <div className="guide-grid">
            <div className="guide-card">
              <strong>1. Escolha a instancia</strong>
              <p>Use o mesmo nome cadastrado no EvolutionGO.</p>
            </div>
            <div className="guide-card">
              <strong>2. Escolha um perfil</strong>
              <p>Comece por um perfil pronto. O avancado fica escondido.</p>
            </div>
            <div className="guide-card">
              <strong>3. Copie o webhook</strong>
              <p>Essa e a URL que deve ser configurada no provider.</p>
            </div>
          </div>

          <form className="settings-form" onSubmit={handleSaveInstanceSettings}>
            <label className="field">
              <span>Instancia</span>
              <input
                value={settingsInstanceId}
                onChange={(event) => setSettingsInstanceId(event.target.value)}
                placeholder="Ex.: servidoron_2_1"
                required
              />
            </label>

            <div className="settings-actions">
              <button className="ghost-btn" type="button" onClick={handleReloadSettings}>
                Carregar configuracao
              </button>
            </div>

            <div className="field field-full">
              <span>Perfil de envio</span>
              <div className="preset-row">
                <button
                  type="button"
                  className={`preset-btn ${settingsPreset === "conservative" ? "active" : ""}`}
                  onClick={() => applyPreset("conservative")}
                >
                  Conservador
                </button>
                <button
                  type="button"
                  className={`preset-btn ${settingsPreset === "balanced" ? "active" : ""}`}
                  onClick={() => applyPreset("balanced")}
                >
                  Equilibrado
                </button>
                <button
                  type="button"
                  className={`preset-btn ${settingsPreset === "faster" ? "active" : ""}`}
                  onClick={() => applyPreset("faster")}
                >
                  Mais rapido
                </button>
              </div>
            </div>

            <label className="toggle-field field-full">
              <input
                type="checkbox"
                checked={settingsForm.humanizer_enabled}
                onChange={(event) =>
                  setSettingsForm((current) => ({
                    ...current,
                    humanizer_enabled: event.target.checked,
                  }))
                }
              />
              <span>Cadencia humana ativa</span>
            </label>

            <label className="toggle-field field-full">
              <input
                type="checkbox"
                checked={settingsForm.webhook_enabled}
                onChange={(event) =>
                  setSettingsForm((current) => ({
                    ...current,
                    webhook_enabled: event.target.checked,
                  }))
                }
              />
              <span>Webhook de producao ativo</span>
            </label>

            <label className="field field-full">
              <span>Eventos do webhook</span>
              <input
                value={settingsForm.webhook_subscriptions}
                onChange={(event) =>
                  setSettingsForm((current) => ({
                    ...current,
                    webhook_subscriptions: event.target.value,
                  }))
                }
                placeholder="ALL"
              />
            </label>

            <div className="field field-full">
              <button
                type="button"
                className="ghost-btn"
                onClick={() => setShowAdvancedSettings((current) => !current)}
              >
                {showAdvancedSettings ? "Esconder ajustes avancados" : "Mostrar ajustes avancados"}
              </button>
            </div>

            {showAdvancedSettings ? (
              <>
                <SettingsInput
                  label="Atraso inicial min (s)"
                  value={settingsForm.initial_delay_min_seconds}
                  onChange={(value) =>
                    setSettingsForm((current) => ({ ...current, initial_delay_min_seconds: value }))
                  }
                />
                <SettingsInput
                  label="Atraso inicial max (s)"
                  value={settingsForm.initial_delay_max_seconds}
                  onChange={(value) =>
                    setSettingsForm((current) => ({ ...current, initial_delay_max_seconds: value }))
                  }
                />
                <SettingsInput
                  label="Intervalo min (s)"
                  value={settingsForm.base_delay_min_seconds}
                  onChange={(value) =>
                    setSettingsForm((current) => ({ ...current, base_delay_min_seconds: value }))
                  }
                />
                <SettingsInput
                  label="Intervalo max (s)"
                  value={settingsForm.base_delay_max_seconds}
                  onChange={(value) =>
                    setSettingsForm((current) => ({ ...current, base_delay_max_seconds: value }))
                  }
                />
                <SettingsInput
                  label="Atraso extra interno min (ms)"
                  value={settingsForm.provider_delay_min_ms}
                  onChange={(value) =>
                    setSettingsForm((current) => ({ ...current, provider_delay_min_ms: value }))
                  }
                />
                <SettingsInput
                  label="Atraso extra interno max (ms)"
                  value={settingsForm.provider_delay_max_ms}
                  onChange={(value) =>
                    setSettingsForm((current) => ({ ...current, provider_delay_max_ms: value }))
                  }
                />
                <SettingsInput
                  label="Bloco min"
                  value={settingsForm.burst_size_min}
                  min="1"
                  onChange={(value) =>
                    setSettingsForm((current) => ({ ...current, burst_size_min: value }))
                  }
                />
                <SettingsInput
                  label="Bloco max"
                  value={settingsForm.burst_size_max}
                  min="1"
                  onChange={(value) =>
                    setSettingsForm((current) => ({ ...current, burst_size_max: value }))
                  }
                />
                <SettingsInput
                  label="Pausa longa min (s)"
                  value={settingsForm.burst_pause_min_seconds}
                  onChange={(value) =>
                    setSettingsForm((current) => ({ ...current, burst_pause_min_seconds: value }))
                  }
                />
                <SettingsInput
                  label="Pausa longa max (s)"
                  value={settingsForm.burst_pause_max_seconds}
                  onChange={(value) =>
                    setSettingsForm((current) => ({ ...current, burst_pause_max_seconds: value }))
                  }
                />
              </>
            ) : null}

            <div className="form-footer">
              <div className="muted">
                Para a maioria dos casos, o perfil Equilibrado ja funciona bem.
              </div>
              <button className="primary-btn" type="submit" disabled={settingsSaving}>
                {settingsSaving ? "Salvando..." : "Salvar configuracao"}
              </button>
            </div>
          </form>

          {showAdvancedSettings ? (
            <div className="advanced-help">
              <strong>Ajustes avancados</strong>
              <p>
                "Atraso extra interno" e um pequeno tempo adicional usado pelo sistema para
                deixar o envio menos repetitivo. Na maioria dos casos, voce nao precisa mexer nisso.
              </p>
            </div>
          ) : null}

          {settingsData ? (
            <div className="webhook-card">
              <div className="section-label">Webhook de producao</div>
              <div className="base-box webhook-box">
                <div>
                  <strong>URL</strong>
                  <div className="muted break-all">{settingsData.webhook_url || "-"}</div>
                </div>
                <div>
                  <strong>Token</strong>
                  <div className="muted break-all">
                    {showWebhookToken ? settingsData.webhook_token || "-" : "Oculto por seguranca"}
                  </div>
                </div>
                <div>
                  <strong>Eventos recomendados</strong>
                  <div className="muted">
                    {(settingsData.webhook_subscriptions || []).join(", ") || "-"}
                  </div>
                </div>
                <div className="inline-actions">
                  <button className="ghost-btn" type="button" onClick={handleCopyWebhook}>
                    Copiar URL
                  </button>
                  <button
                    className="ghost-btn"
                    type="button"
                    onClick={() => setShowWebhookToken((current) => !current)}
                  >
                    {showWebhookToken ? "Esconder token" : "Mostrar token"}
                  </button>
                </div>
              </div>
              <div className="muted webhook-help">
                Configure essa URL no EvolutionGO para a instancia selecionada. Se quiser
                simplificar a configuracao, use o evento ALL no provider.
              </div>
            </div>
          ) : null}

          {settingsLoading ? <div className="muted small">Carregando configuracao...</div> : null}
          {settingsSuccess ? <div className="success-banner">{settingsSuccess}</div> : null}
        </div>
      </section>
      ) : null}

      {error ? <div className="alert">{error}</div> : null}

      {activeView === "operations" ? (
      <section className="layout">
        <div className="panel">
          <div className="card-head">
            <div>
              <div className="card-title">Campanhas recentes</div>
              <div className="muted">{refreshStamp || "Sincronizando..."}</div>
            </div>
            <button
              className="ghost-btn"
              onClick={() => refreshAll(session.token, false)}
              disabled={loading}
            >
              Atualizar
            </button>
          </div>
          <div className="card-body card-scroll">
            {loading && !campaigns.length ? (
              <div className="empty">Carregando campanhas...</div>
            ) : campaigns.length ? (
              <div className="campaign-list">
                {campaigns.map((campaign) => (
                  <button
                    key={campaign.id}
                    className={`campaign-item ${selectedCampaign?.id === campaign.id ? "active" : ""}`}
                    onClick={() => loadCampaign(campaign.id)}
                  >
                    <div className="campaign-row">
                      <strong>{campaign.name}</strong>
                      <StatusChip status={campaign.status} />
                    </div>
                    <div className="muted small">
                      Instancia {campaign.instance_id} | Total {campaign.total_messages}
                    </div>
                    <div className="muted small">
                      Enviadas {campaign.sent_count} | Entregues {campaign.delivered_count} |
                      Lidas {campaign.read_count} | Falhas {campaign.failed_count}
                    </div>
                  </button>
                ))}
              </div>
            ) : (
              <div className="empty">Ainda nao existem campanhas cadastradas.</div>
            )}
          </div>
        </div>

        <div className="panel">
          <div className="card-head">
            <div className="card-title">Detalhes operacionais</div>
          </div>
          <div className="card-body card-scroll">
            {selectedCampaign ? (
              <>
                <div className="detail-header">
                  <div>
                    <h2>{selectedCampaign.name}</h2>
                    <div className="muted">Instancia {selectedCampaign.instance_id}</div>
                    <div className="muted">Criada em {formatDate(selectedCampaign.created_at)}</div>
                  </div>
                  <StatusChip status={selectedCampaign.status} />
                </div>

                <div className="kpi-grid">
                  <Kpi label="Pendentes" value={selectedCampaign.pending_count} />
                  <Kpi label="Processando" value={selectedCampaign.processing_count} />
                  <Kpi label="Enviadas" value={selectedCampaign.sent_count} />
                  <Kpi label="Entregues" value={selectedCampaign.delivered_count} />
                  <Kpi label="Lidas" value={selectedCampaign.read_count} />
                  <Kpi label="Falhas" value={selectedCampaign.failed_count} />
                </div>

                <div className="action-row">
                  <button
                    className="primary-btn"
                    onClick={() => handleCampaignAction("pause")}
                    disabled={!canPause || actionLoading}
                  >
                    Pausar
                  </button>
                  <button
                    className="secondary-btn"
                    onClick={() => handleCampaignAction("resume")}
                    disabled={!canResume || actionLoading}
                  >
                    Retomar
                  </button>
                  <button
                    className="ghost-btn"
                    onClick={() => loadCampaign(selectedCampaign.id)}
                    disabled={actionLoading}
                  >
                    Recarregar
                  </button>
                </div>

                <div className="message-base">
                  <div className="section-label">Mensagem base</div>
                  <div className="base-box">{selectedCampaign.message}</div>
                </div>

                <div className="message-list">
                  {messages.length ? (
                    messages.map((message) => (
                      <div className="message-card" key={message.id}>
                        <div className="campaign-row">
                          <strong>{message.recipient_phone}</strong>
                          <StatusChip status={message.status} />
                        </div>
                        <div className="muted small">
                          Tentativas {message.attempt_count} | Provider{" "}
                          {message.provider_message_id || "-"}
                        </div>
                        <div className="muted small">
                          Retry {formatDate(message.next_retry_at)} | Enviado{" "}
                          {formatDate(message.sent_at)}
                        </div>
                        <div className="muted small">
                          Entregue {formatDate(message.delivered_at)} | Lido{" "}
                          {formatDate(message.read_at)}
                        </div>
                        {message.last_error ? (
                          <div className="error-line">Erro: {message.last_error}</div>
                        ) : null}
                      </div>
                    ))
                  ) : (
                    <div className="empty">Nenhuma mensagem encontrada para esta campanha.</div>
                  )}
                </div>
              </>
            ) : (
              <div className="empty">Selecione uma campanha para ver os detalhes.</div>
            )}
          </div>
        </div>
      </section>
      ) : null}
        </main>
      </div>
    </div>
  );
}

function StatCard({ label, value }) {
  return (
    <div className="stat-card">
      <div className="stat-label">{label}</div>
      <div className="stat-value">{value}</div>
    </div>
  );
}

function Kpi({ label, value }) {
  return (
    <div className="kpi-card">
      <b>{value}</b>
      <span>{label}</span>
    </div>
  );
}

function SettingsInput({ label, value, onChange, min = "0" }) {
  return (
    <label className="field">
      <span>{label}</span>
      <input type="number" min={min} value={value} onChange={(event) => onChange(event.target.value)} required />
    </label>
  );
}

function StatusChip({ status }) {
  return <span className={`status-chip ${statusClass(status)}`}>{status}</span>;
}

export default App;


