import { useEffect, useMemo, useRef, useState } from "react";

const AUTH_STORAGE_KEY = "disparago.auth";
const POLL_INTERVAL_MS = 8000;
const ACTIVE_STATUSES = new Set(["pending", "processing", "paused"]);
const FINISHED_STATUSES = new Set(["sent", "delivered", "read", "failed", "partial"]);

async function request(url, options = {}, token) {
  const headers = new Headers(options.headers || {});
  if (token) headers.set("Authorization", `Bearer ${token}`);

  const response = await fetch(url, { ...options, headers });
  const text = await response.text();
  let payload = {};

  if (text) {
    try {
      payload = JSON.parse(text);
    } catch {
      payload = { error: text };
    }
  }

  if (!response.ok) {
    const error = new Error(payload.error || "Falha na requisicao");
    error.status = response.status;
    throw error;
  }

  return payload;
}

function loadStoredSession() {
  try {
    const raw = window.localStorage.getItem(AUTH_STORAGE_KEY);
    return raw ? JSON.parse(raw) : null;
  } catch {
    window.localStorage.removeItem(AUTH_STORAGE_KEY);
    return null;
  }
}

function saveSession(session) {
  window.localStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify(session));
}

function clearSession() {
  window.localStorage.removeItem(AUTH_STORAGE_KEY);
}

function formatDate(value, timezone) {
  if (!value) return "-";
  try {
    return new Intl.DateTimeFormat("pt-BR", {
      dateStyle: "short",
      timeStyle: "short",
      timeZone: timezone || undefined,
    }).format(new Date(value));
  } catch {
    return new Date(value).toLocaleString("pt-BR");
  }
}

function statusLabel(status) {
  return (
    {
      scheduled: "Agendada",
      pending: "Pendente",
      processing: "Em andamento",
      paused: "Pausada",
      sent: "Enviada",
      delivered: "Entregue",
      read: "Lida",
      failed: "Falhou",
      partial: "Parcial",
      draft: "Rascunho",
    }[(status || "").toLowerCase()] || status || "Pendente"
  );
}

function statusClass(status) {
  return (status || "pending").toLowerCase();
}

function initialCampaignForm() {
  return {
    name: "",
    instanceId: "",
    message: "",
    contacts: "",
    sendMode: "now",
    scheduledAt: "",
    timezone: "America/Sao_Paulo",
  };
}

function initialAdminForm() {
  return {
    companyId: "",
    instanceId: "",
    humanizerEnabled: true,
    initialDelayMinSec: "4",
    initialDelayMaxSec: "10",
    baseDelayMinSec: "12",
    baseDelayMaxSec: "24",
    providerDelayMinMs: "2000",
    providerDelayMaxMs: "6000",
    burstSizeMin: "3",
    burstSizeMax: "5",
    burstPauseMinSec: "60",
    burstPauseMaxSec: "150",
    webhookEnabled: true,
    webhookSubscriptions: "ALL",
  };
}

function initialRescheduleForm(campaign) {
  return {
    scheduledAt: campaign?.scheduled_at ? new Date(campaign.scheduled_at).toISOString().slice(0, 16) : "",
    timezone: campaign?.scheduled_timezone || "America/Sao_Paulo",
  };
}

function splitCampaigns(campaigns) {
  return {
    scheduled: campaigns.filter((item) => statusClass(item.status) === "scheduled"),
    active: campaigns.filter((item) => ACTIVE_STATUSES.has(statusClass(item.status))),
    finished: campaigns.filter((item) => FINISHED_STATUSES.has(statusClass(item.status))),
  };
}

function aggregateStats(campaigns) {
  return {
    total: campaigns.length,
    scheduled: campaigns.filter((item) => statusClass(item.status) === "scheduled").length,
    active: campaigns.filter((item) => ACTIVE_STATUSES.has(statusClass(item.status))).length,
    finished: campaigns.filter((item) => FINISHED_STATUSES.has(statusClass(item.status))).length,
  };
}

function StepChip({ active, label }) {
  return <span className={`step-chip ${active ? "active" : ""}`}>{label}</span>;
}

function StatusChip({ status }) {
  return <span className={`status-chip ${statusClass(status)}`}>{statusLabel(status)}</span>;
}

function StatCard({ label, value, small = false }) {
  return (
    <div className={`stat-card ${small ? "small" : ""}`}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function SettingsInput({ label, value, onChange }) {
  return (
    <label className="field">
      <span>{label}</span>
      <input type="number" value={value} onChange={(event) => onChange(event.target.value)} required />
    </label>
  );
}

function CampaignColumn({ title, subtitle, items, selectedId, onSelect }) {
  return (
    <section className="panel column-panel">
      <div className="panel-header">
        <div>
          <h2>{title}</h2>
          <p>{subtitle}</p>
        </div>
      </div>
      <div className="column-list">
        {items.length ? (
          items.map((item) => (
            <button
              key={item.id}
              className={`campaign-card ${selectedId === item.id ? "selected" : ""}`}
              onClick={() => onSelect(item.id)}
            >
              <div className="message-top">
                <strong>{item.name}</strong>
                <StatusChip status={item.status} />
              </div>
              <p>Instancia {item.instance_id}</p>
              <p>Total {item.total_messages}</p>
              {item.status === "scheduled" ? (
                <p>{formatDate(item.scheduled_at || item.scheduled_at_utc, item.scheduled_timezone)}</p>
              ) : (
                <p>Enviadas {item.sent_count} · Falhas {item.failed_count}</p>
              )}
            </button>
          ))
        ) : (
          <div className="empty-state">Nenhuma campanha nesta coluna.</div>
        )}
      </div>
    </section>
  );
}

function App() {
  const [session, setSession] = useState(null);
  const [checkingAuth, setCheckingAuth] = useState(true);
  const [authForm, setAuthForm] = useState({ username: "", password: "" });
  const [authLoading, setAuthLoading] = useState(false);
  const [authError, setAuthError] = useState("");
  const [campaigns, setCampaigns] = useState([]);
  const [selectedCampaign, setSelectedCampaign] = useState(null);
  const [messages, setMessages] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [refreshStamp, setRefreshStamp] = useState("");
  const [formStep, setFormStep] = useState(1);
  const [form, setForm] = useState(initialCampaignForm());
  const [createLoading, setCreateLoading] = useState(false);
  const [createSuccess, setCreateSuccess] = useState("");
  const [actionLoading, setActionLoading] = useState(false);
  const [rescheduleOpen, setRescheduleOpen] = useState(false);
  const [rescheduleForm, setRescheduleForm] = useState(initialRescheduleForm());
  const [adminForm, setAdminForm] = useState(initialAdminForm());
  const [adminData, setAdminData] = useState(null);
  const [adminLoading, setAdminLoading] = useState(false);
  const [adminSuccess, setAdminSuccess] = useState("");
  const [currentView, setCurrentView] = useState("operations");
  const selectedIdRef = useRef(null);

  const grouped = useMemo(() => splitCampaigns(campaigns), [campaigns]);
  const stats = useMemo(() => aggregateStats(campaigns), [campaigns]);
  const isSuperadmin = session?.role === "superadmin";

  useEffect(() => {
    const stored = loadStoredSession();
    if (!stored?.token) {
      setCheckingAuth(false);
      setLoading(false);
      return;
    }

    request("/api/v1/auth/me", {}, stored.token)
      .then((response) => {
        const nextSession = { ...stored, ...response.data, token: stored.token };
        saveSession(nextSession);
        setSession(nextSession);
      })
      .catch(() => clearSession())
      .finally(() => setCheckingAuth(false));
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
    setRescheduleForm(initialRescheduleForm(selectedCampaign));
  }, [selectedCampaign?.id]);

  async function refreshAll(token, selectFirst) {
    try {
      const response = await request("/api/v1/campaigns?limit=50", {}, token);
      const nextCampaigns = response.data || [];
      setCampaigns(nextCampaigns);

      let nextSelectedId = selectedIdRef.current;
      if (!nextSelectedId && selectFirst && nextCampaigns.length) nextSelectedId = nextCampaigns[0].id;
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
      setLoading(false);
      setError("");
    } catch (err) {
      if (err.status === 401) return handleLogout("Sua sessao expirou.");
      setError(err.message);
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
      setMessages(messagesResponse.data?.messages || []);
      setError("");
    } catch (err) {
      if (err.status === 401) return handleLogout("Sua sessao expirou.");
      setError(err.message);
    }
  }

  function handleLogout(message = "") {
    clearSession();
    setSession(null);
    setCampaigns([]);
    setSelectedCampaign(null);
    setMessages([]);
    setLoading(false);
    setAuthError(message);
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
      const nextSession = { ...response.data, token: response.data.token };
      saveSession(nextSession);
      setSession(nextSession);
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
      setError("");
      const contacts = form.contacts.split(/\r?\n|,/).map((item) => item.trim()).filter(Boolean);

      const payload = {
        name: form.name.trim(),
        instance_id: form.instanceId.trim(),
        message: form.message.trim(),
        contacts,
        send_mode: form.sendMode,
        scheduled_at: form.sendMode === "scheduled" ? form.scheduledAt : "",
        timezone: form.sendMode === "scheduled" ? form.timezone : "",
      };

      const created = await request("/api/v1/campaigns", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      }, session.token);

      selectedIdRef.current = created.data.id;
      setCreateSuccess(
        payload.send_mode === "scheduled"
          ? `Campanha agendada para ${formatDate(payload.scheduled_at, payload.timezone)}.`
          : `Campanha criada com ${created.data.total_messages} contatos.`
      );
      setForm(initialCampaignForm());
      setFormStep(1);
      await refreshAll(session.token, false);
    } catch (err) {
      if (err.status === 401) return handleLogout("Sua sessao expirou.");
      setError(err.message);
    } finally {
      setCreateLoading(false);
    }
  }

  async function handleCampaignAction(action) {
    if (!selectedCampaign) return;
    try {
      setActionLoading(true);
      await request(`/api/v1/campaigns/${selectedCampaign.id}/${action}`, { method: "POST" }, session.token);
      await refreshAll(session.token, false);
    } catch (err) {
      if (err.status === 401) return handleLogout("Sua sessao expirou.");
      setError(err.message);
    } finally {
      setActionLoading(false);
    }
  }

  async function handleReschedule(event) {
    event.preventDefault();
    if (!selectedCampaign) return;
    try {
      setActionLoading(true);
      await request(`/api/v1/campaigns/${selectedCampaign.id}/reschedule`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          scheduled_at: rescheduleForm.scheduledAt,
          timezone: rescheduleForm.timezone,
        }),
      }, session.token);
      setRescheduleOpen(false);
      await refreshAll(session.token, false);
    } catch (err) {
      if (err.status === 401) return handleLogout("Sua sessao expirou.");
      setError(err.message);
    } finally {
      setActionLoading(false);
    }
  }

  async function loadAdminSettings() {
    try {
      setAdminLoading(true);
      const response = await request(
        `/api/v1/admin/companies/${adminForm.companyId}/instances/${adminForm.instanceId}/settings`,
        {},
        session.token
      );
      const data = response.data;
      setAdminData(data);
      setAdminForm((current) => ({
        ...current,
        humanizerEnabled: data.humanizer_enabled,
        initialDelayMinSec: String(data.initial_delay_min_seconds),
        initialDelayMaxSec: String(data.initial_delay_max_seconds),
        baseDelayMinSec: String(data.base_delay_min_seconds),
        baseDelayMaxSec: String(data.base_delay_max_seconds),
        providerDelayMinMs: String(data.provider_delay_min_ms),
        providerDelayMaxMs: String(data.provider_delay_max_ms),
        burstSizeMin: String(data.burst_size_min),
        burstSizeMax: String(data.burst_size_max),
        burstPauseMinSec: String(data.burst_pause_min_seconds),
        burstPauseMaxSec: String(data.burst_pause_max_seconds),
        webhookEnabled: data.webhook_enabled,
        webhookSubscriptions: (data.webhook_subscriptions || []).join(", "),
      }));
      setAdminSuccess("");
    } catch (err) {
      if (err.status === 401) return handleLogout("Sua sessao expirou.");
      setError(err.message);
    } finally {
      setAdminLoading(false);
    }
  }

  async function saveAdminSettings(event) {
    event.preventDefault();
    try {
      setAdminLoading(true);
      const response = await request(
        `/api/v1/admin/companies/${adminForm.companyId}/instances/${adminForm.instanceId}/settings`,
        {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            company_id: Number(adminForm.companyId),
            instance_id: adminForm.instanceId,
            humanizer_enabled: adminForm.humanizerEnabled,
            initial_delay_min_seconds: Number(adminForm.initialDelayMinSec),
            initial_delay_max_seconds: Number(adminForm.initialDelayMaxSec),
            base_delay_min_seconds: Number(adminForm.baseDelayMinSec),
            base_delay_max_seconds: Number(adminForm.baseDelayMaxSec),
            provider_delay_min_ms: Number(adminForm.providerDelayMinMs),
            provider_delay_max_ms: Number(adminForm.providerDelayMaxMs),
            burst_size_min: Number(adminForm.burstSizeMin),
            burst_size_max: Number(adminForm.burstSizeMax),
            burst_pause_min_seconds: Number(adminForm.burstPauseMinSec),
            burst_pause_max_seconds: Number(adminForm.burstPauseMaxSec),
            webhook_enabled: adminForm.webhookEnabled,
            webhook_subscriptions: adminForm.webhookSubscriptions
              .split(",")
              .map((item) => item.trim())
              .filter(Boolean),
          }),
        },
        session.token
      );
      setAdminData(response.data);
      setAdminSuccess("Configuracao salva com sucesso.");
    } catch (err) {
      if (err.status === 401) return handleLogout("Sua sessao expirou.");
      setError(err.message);
    } finally {
      setAdminLoading(false);
    }
  }

  if (checkingAuth) return <div className="shell centered">Validando sessao...</div>;

  if (!session) {
    return (
      <div className="shell auth-shell">
        <form className="auth-card" onSubmit={handleLogin}>
          <div className="eyebrow">DisparaGO</div>
          <h1>Painel de campanhas</h1>
          <p>Entre com seu usuario para ver apenas as campanhas da sua empresa.</p>
          <label className="field">
            <span>Usuario</span>
            <input value={authForm.username} onChange={(event) => setAuthForm((current) => ({ ...current, username: event.target.value }))} required />
          </label>
          <label className="field">
            <span>Senha</span>
            <input type="password" value={authForm.password} onChange={(event) => setAuthForm((current) => ({ ...current, password: event.target.value }))} required />
          </label>
          <button className="primary-btn" type="submit" disabled={authLoading}>
            {authLoading ? "Entrando..." : "Entrar"}
          </button>
          {authError ? <div className="alert">{authError}</div> : null}
        </form>
      </div>
    );
  }

  return (
    <div className="shell">
      <header className="topbar">
        <div>
          <div className="eyebrow">Operacao de campanhas</div>
          <h1>DisparaGO</h1>
          <p>
            {session.display_name || session.username}
            {session.company_name ? ` · ${session.company_name}` : ""}
          </p>
        </div>
        <div className="topbar-actions">
          <button className={`ghost-btn ${currentView === "operations" ? "active" : ""}`} onClick={() => setCurrentView("operations")}>Operacao</button>
          {isSuperadmin ? (
            <button className={`ghost-btn ${currentView === "admin" ? "active" : ""}`} onClick={() => setCurrentView("admin")}>Superadmin</button>
          ) : null}
          <button className="ghost-btn" onClick={() => handleLogout()}>Sair</button>
        </div>
      </header>

      {currentView === "operations" ? (
        <>
          <section className="stats-grid">
            <StatCard label="Total" value={stats.total} />
            <StatCard label="Agendadas" value={stats.scheduled} />
            <StatCard label="Em andamento" value={stats.active} />
            <StatCard label="Finalizadas" value={stats.finished} />
          </section>

          <section className="panel wizard-panel">
            <div className="panel-header">
              <div>
                <h2>Nova campanha</h2>
                <p>Fluxo simples para criar e, se quiser, agendar o disparo.</p>
              </div>
              <div className="step-row">
                <StepChip active={formStep === 1} label="1. Base" />
                <StepChip active={formStep === 2} label="2. Contatos" />
                <StepChip active={formStep === 3} label="3. Envio" />
              </div>
            </div>

            <form className="wizard-form" onSubmit={handleCreateCampaign}>
              {formStep === 1 ? (
                <div className="form-grid">
                  <label className="field">
                    <span>Nome da campanha</span>
                    <input value={form.name} onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))} placeholder="Ex.: Promocao Abril" required />
                  </label>
                  <label className="field">
                    <span>Instancia WhatsApp</span>
                    <input value={form.instanceId} onChange={(event) => setForm((current) => ({ ...current, instanceId: event.target.value }))} placeholder="Ex.: servidoron_2_1" required />
                  </label>
                  <label className="field full">
                    <span>Mensagem</span>
                    <textarea rows={5} value={form.message} onChange={(event) => setForm((current) => ({ ...current, message: event.target.value }))} placeholder="Digite a mensagem que sera enviada" required />
                  </label>
                </div>
              ) : null}

              {formStep === 2 ? (
                <div className="form-grid">
                  <label className="field full">
                    <span>Contatos</span>
                    <textarea rows={8} value={form.contacts} onChange={(event) => setForm((current) => ({ ...current, contacts: event.target.value }))} placeholder={"Um numero por linha\n5511999999999\n5511888888888"} required />
                  </label>
                  <div className="helper-box full">Aceita um numero por linha ou separados por virgula.</div>
                </div>
              ) : null}

              {formStep === 3 ? (
                <div className="form-grid">
                  <div className="mode-grid full">
                    <button type="button" className={`mode-card ${form.sendMode === "now" ? "active" : ""}`} onClick={() => setForm((current) => ({ ...current, sendMode: "now" }))}>
                      <strong>Enviar agora</strong>
                      <span>Entra na fila assim que a campanha for criada.</span>
                    </button>
                    <button type="button" className={`mode-card ${form.sendMode === "scheduled" ? "active" : ""}`} onClick={() => setForm((current) => ({ ...current, sendMode: "scheduled" }))}>
                      <strong>Agendar</strong>
                      <span>Fica aguardando e so entra na fila no horario escolhido.</span>
                    </button>
                  </div>

                  {form.sendMode === "scheduled" ? (
                    <>
                      <label className="field">
                        <span>Data e hora</span>
                        <input type="datetime-local" value={form.scheduledAt} onChange={(event) => setForm((current) => ({ ...current, scheduledAt: event.target.value }))} required />
                      </label>
                      <label className="field">
                        <span>Fuso</span>
                        <input value={form.timezone} onChange={(event) => setForm((current) => ({ ...current, timezone: event.target.value }))} required />
                      </label>
                      <div className="helper-box full">
                        {form.scheduledAt ? `Agendada para ${formatDate(form.scheduledAt, form.timezone)} (${form.timezone}).` : "Escolha data, hora e fuso para programar o envio."}
                      </div>
                    </>
                  ) : (
                    <div className="helper-box full">A campanha sera criada e enviada para o worker assim que voce confirmar.</div>
                  )}
                </div>
              ) : null}

              <div className="wizard-footer">
                <button type="button" className="ghost-btn" onClick={() => setFormStep((current) => Math.max(1, current - 1))} disabled={formStep === 1}>Voltar</button>
                {formStep < 3 ? (
                  <button type="button" className="primary-btn" onClick={() => setFormStep((current) => Math.min(3, current + 1))}>Continuar</button>
                ) : (
                  <button className="primary-btn" type="submit" disabled={createLoading}>
                    {createLoading ? "Salvando..." : form.sendMode === "scheduled" ? "Agendar campanha" : "Criar campanha"}
                  </button>
                )}
              </div>
            </form>
            {createSuccess ? <div className="success-banner">{createSuccess}</div> : null}
          </section>

          <section className="layout">
            <CampaignColumn title="Agendadas" subtitle="Aguardando o horario de envio" items={grouped.scheduled} selectedId={selectedCampaign?.id} onSelect={loadCampaign} />
            <CampaignColumn title="Em andamento" subtitle="Campanhas que estao processando agora" items={grouped.active} selectedId={selectedCampaign?.id} onSelect={loadCampaign} />
            <CampaignColumn title="Finalizadas" subtitle="Campanhas concluidas ou com falha" items={grouped.finished} selectedId={selectedCampaign?.id} onSelect={loadCampaign} />
          </section>
        </>
      ) : null}

      {currentView === "operations" ? (
        <section className="panel detail-panel">
          <div className="panel-header">
            <div>
              <h2>Detalhes da campanha</h2>
              <p>{refreshStamp || "Sincronizando..."}</p>
            </div>
            <button className="ghost-btn" onClick={() => refreshAll(session.token, false)}>Atualizar</button>
          </div>

          {!selectedCampaign ? (
            <div className="empty-state">Selecione uma campanha para ver os detalhes.</div>
          ) : (
            <>
              <div className="detail-head">
                <div>
                  <h3>{selectedCampaign.name}</h3>
                  <p>Instancia {selectedCampaign.instance_id} · criada em {formatDate(selectedCampaign.created_at)}</p>
                  {selectedCampaign.status === "scheduled" ? (
                    <p>
                      Agendada para {formatDate(selectedCampaign.scheduled_at || selectedCampaign.scheduled_at_utc, selectedCampaign.scheduled_timezone)} ({selectedCampaign.scheduled_timezone || "UTC"})
                    </p>
                  ) : null}
                </div>
                <StatusChip status={selectedCampaign.status} />
              </div>

              <div className="kpi-grid">
                <StatCard label="Pendentes" value={selectedCampaign.pending_count} small />
                <StatCard label="Processando" value={selectedCampaign.processing_count} small />
                <StatCard label="Enviadas" value={selectedCampaign.sent_count} small />
                <StatCard label="Entregues" value={selectedCampaign.delivered_count} small />
                <StatCard label="Lidas" value={selectedCampaign.read_count} small />
                <StatCard label="Falhas" value={selectedCampaign.failed_count} small />
              </div>

              <div className="action-row">
                <button className="primary-btn" disabled={actionLoading || selectedCampaign.paused || selectedCampaign.status === "scheduled" || FINISHED_STATUSES.has(statusClass(selectedCampaign.status))} onClick={() => handleCampaignAction("pause")}>Pausar</button>
                <button className="secondary-btn" disabled={actionLoading || !selectedCampaign.paused} onClick={() => handleCampaignAction("resume")}>Retomar</button>
                {selectedCampaign.status === "scheduled" ? (
                  <>
                    <button className="ghost-btn" onClick={() => setRescheduleOpen((current) => !current)}>{rescheduleOpen ? "Fechar reagendamento" : "Reagendar"}</button>
                    <button className="danger-btn" disabled={actionLoading} onClick={() => handleCampaignAction("cancel")}>Cancelar agenda</button>
                  </>
                ) : null}
              </div>

              {rescheduleOpen && selectedCampaign.status === "scheduled" ? (
                <form className="inline-form" onSubmit={handleReschedule}>
                  <label className="field">
                    <span>Nova data e hora</span>
                    <input type="datetime-local" value={rescheduleForm.scheduledAt} onChange={(event) => setRescheduleForm((current) => ({ ...current, scheduledAt: event.target.value }))} required />
                  </label>
                  <label className="field">
                    <span>Fuso</span>
                    <input value={rescheduleForm.timezone} onChange={(event) => setRescheduleForm((current) => ({ ...current, timezone: event.target.value }))} required />
                  </label>
                  <button className="primary-btn" type="submit" disabled={actionLoading}>Salvar novo horario</button>
                </form>
              ) : null}

              <div className="message-box">
                <div className="section-title">Mensagem base</div>
                <div className="message-content">{selectedCampaign.message}</div>
              </div>

              <div className="section-title">Mensagens</div>
              <div className="message-list">
                {messages.length ? messages.map((item) => (
                  <article className="message-card" key={item.id}>
                    <div className="message-top">
                      <strong>{item.recipient_phone}</strong>
                      <StatusChip status={item.status} />
                    </div>
                    <p>Tentativas: {item.attempt_count}</p>
                    <p>Enviado: {formatDate(item.sent_at)}</p>
                    <p>Entregue: {formatDate(item.delivered_at)}</p>
                    <p>Lido: {formatDate(item.read_at)}</p>
                    {item.last_error ? <p className="error-line">Erro: {item.last_error}</p> : null}
                  </article>
                )) : <div className="empty-state">Nenhuma mensagem encontrada para esta campanha.</div>}
              </div>
            </>
          )}
        </section>
      ) : (
        <section className="panel admin-panel">
          <div className="panel-header">
            <div>
              <h2>Configuracao por instancia</h2>
              <p>Area reservada para superadmin por empresa e instancia.</p>
            </div>
          </div>

          <form className="form-grid" onSubmit={saveAdminSettings}>
            <label className="field">
              <span>Empresa</span>
              <input value={adminForm.companyId} onChange={(event) => setAdminForm((current) => ({ ...current, companyId: event.target.value }))} required />
            </label>
            <label className="field">
              <span>Instancia</span>
              <input value={adminForm.instanceId} onChange={(event) => setAdminForm((current) => ({ ...current, instanceId: event.target.value }))} required />
            </label>
            <div className="action-row full">
              <button className="ghost-btn" type="button" onClick={loadAdminSettings}>Carregar configuracao</button>
            </div>
            <label className="toggle-field full">
              <input type="checkbox" checked={adminForm.humanizerEnabled} onChange={(event) => setAdminForm((current) => ({ ...current, humanizerEnabled: event.target.checked }))} />
              <span>Cadencia humana ativa</span>
            </label>
            <SettingsInput label="Atraso inicial min (s)" value={adminForm.initialDelayMinSec} onChange={(value) => setAdminForm((current) => ({ ...current, initialDelayMinSec: value }))} />
            <SettingsInput label="Atraso inicial max (s)" value={adminForm.initialDelayMaxSec} onChange={(value) => setAdminForm((current) => ({ ...current, initialDelayMaxSec: value }))} />
            <SettingsInput label="Intervalo min (s)" value={adminForm.baseDelayMinSec} onChange={(value) => setAdminForm((current) => ({ ...current, baseDelayMinSec: value }))} />
            <SettingsInput label="Intervalo max (s)" value={adminForm.baseDelayMaxSec} onChange={(value) => setAdminForm((current) => ({ ...current, baseDelayMaxSec: value }))} />
            <SettingsInput label="Atraso extra interno min (ms)" value={adminForm.providerDelayMinMs} onChange={(value) => setAdminForm((current) => ({ ...current, providerDelayMinMs: value }))} />
            <SettingsInput label="Atraso extra interno max (ms)" value={adminForm.providerDelayMaxMs} onChange={(value) => setAdminForm((current) => ({ ...current, providerDelayMaxMs: value }))} />
            <SettingsInput label="Bloco min" value={adminForm.burstSizeMin} onChange={(value) => setAdminForm((current) => ({ ...current, burstSizeMin: value }))} />
            <SettingsInput label="Bloco max" value={adminForm.burstSizeMax} onChange={(value) => setAdminForm((current) => ({ ...current, burstSizeMax: value }))} />
            <SettingsInput label="Pausa longa min (s)" value={adminForm.burstPauseMinSec} onChange={(value) => setAdminForm((current) => ({ ...current, burstPauseMinSec: value }))} />
            <SettingsInput label="Pausa longa max (s)" value={adminForm.burstPauseMaxSec} onChange={(value) => setAdminForm((current) => ({ ...current, burstPauseMaxSec: value }))} />
            <label className="toggle-field full">
              <input type="checkbox" checked={adminForm.webhookEnabled} onChange={(event) => setAdminForm((current) => ({ ...current, webhookEnabled: event.target.checked }))} />
              <span>Webhook ativo</span>
            </label>
            <label className="field full">
              <span>Eventos do webhook</span>
              <input value={adminForm.webhookSubscriptions} onChange={(event) => setAdminForm((current) => ({ ...current, webhookSubscriptions: event.target.value }))} />
            </label>
            <div className="action-row full">
              <button className="primary-btn" type="submit" disabled={adminLoading}>{adminLoading ? "Salvando..." : "Salvar configuracao"}</button>
            </div>
          </form>
          {adminData ? <div className="helper-box">URL do webhook: {adminData.webhook_url}</div> : null}
          {adminSuccess ? <div className="success-banner">{adminSuccess}</div> : null}
        </section>
      )}

      {error ? <div className="alert floating">{error}</div> : null}
    </div>
  );
}

export default App;
