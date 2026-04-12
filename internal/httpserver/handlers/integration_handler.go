package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	authdomain "disparago/internal/domain/auth"
	"disparago/internal/service"
)

type IntegrationHandler struct {
	authService     *service.AuthService
	campaignService *service.CampaignService
}

type InternalCampaignRequest struct {
	CompanyName        string   `json:"company_name"`
	CompanyExternalID  string   `json:"company_external_id"`
	UserExternalID     string   `json:"user_external_id"`
	Username           string   `json:"username"`
	DisplayName        string   `json:"display_name"`
	Name               string   `json:"name"`
	InstanceID         string   `json:"instance_id"`
	Message            string   `json:"message"`
	Contacts           []string `json:"contacts"`
	SendMode           string   `json:"send_mode"`
	ScheduledAt        string   `json:"scheduled_at"`
	Timezone           string   `json:"timezone"`
	ExternalCampaignID string   `json:"external_campaign_id"`
}

func NewIntegrationHandler(authService *service.AuthService, campaignService *service.CampaignService) *IntegrationHandler {
	return &IntegrationHandler{
		authService:     authService,
		campaignService: campaignService,
	}
}

func (h *IntegrationHandler) UpsertCompany(c *fiber.Ctx) error {
	var req struct {
		Name       string `json:"name"`
		ExternalID string `json:"external_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	company, err := h.authService.UpsertCompany(c.UserContext(), service.SyncCompanyInput{
		Name:           req.Name,
		ExternalSource: "servidoron",
		ExternalID:     req.ExternalID,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"data": company})
}

func (h *IntegrationHandler) UpsertUser(c *fiber.Ctx) error {
	var req struct {
		CompanyID   int64  `json:"company_id"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
		Role        string `json:"role"`
		ExternalID  string `json:"external_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	var companyID *int64
	if req.CompanyID > 0 {
		companyID = &req.CompanyID
	}

	role := authdomain.Role(req.Role)
	if role == "" {
		role = authdomain.RoleOperator
	}

	user, err := h.authService.UpsertUser(c.UserContext(), service.SyncUserInput{
		CompanyID:      companyID,
		Username:       req.Username,
		DisplayName:    req.DisplayName,
		Password:       req.Password,
		Role:           role,
		Active:         true,
		ExternalSource: "servidoron",
		ExternalID:     req.ExternalID,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"data": user})
}

func (h *IntegrationHandler) CreateOrUpdateCampaign(c *fiber.Ctx) error {
	var req InternalCampaignRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	company, err := h.authService.UpsertCompany(c.UserContext(), service.SyncCompanyInput{
		Name:           req.CompanyName,
		ExternalSource: "servidoron",
		ExternalID:     req.CompanyExternalID,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var createdBy *int64
	if req.Username != "" || req.UserExternalID != "" {
		user, err := h.authService.UpsertUser(c.UserContext(), service.SyncUserInput{
			CompanyID:      &company.ID,
			Username:       req.Username,
			DisplayName:    req.DisplayName,
			Password:       req.UserExternalID,
			Role:           authdomain.RoleOperator,
			Active:         true,
			ExternalSource: "servidoron",
			ExternalID:     req.UserExternalID,
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		createdBy = &user.ID
	}

	result, err := h.campaignService.Create(c.UserContext(), service.CreateCampaignInput{
		CompanyID:        company.ID,
		CreatedByUserID:  createdBy,
		Name:             req.Name,
		InstanceID:       req.InstanceID,
		Message:          req.Message,
		Contacts:         req.Contacts,
		SendMode:         req.SendMode,
		ScheduledAt:      req.ScheduledAt,
		Timezone:         req.Timezone,
		ExternalSource:   "servidoron",
		ExternalSourceID: req.ExternalCampaignID,
	})
	if err != nil {
		return campaignError(c, err)
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": "campaign synced",
		"data":    result,
	})
}

func (h *IntegrationHandler) RescheduleCampaign(c *fiber.Ctx) error {
	companyID, err := strconv.ParseInt(c.Query("company_id"), 10, 64)
	if err != nil || companyID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid company_id"})
	}

	var req struct {
		ScheduledAt string `json:"scheduled_at"`
		Timezone    string `json:"timezone"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	record, err := h.campaignService.Reschedule(c.UserContext(), authdomain.Actor{
		CompanyID: companyID,
		Role:      authdomain.RoleOperator,
	}, service.UpdateScheduleInput{
		CampaignID:  c.Params("id"),
		ScheduledAt: req.ScheduledAt,
		Timezone:    req.Timezone,
	})
	if err != nil {
		return campaignError(c, err)
	}

	return c.JSON(fiber.Map{"data": record})
}

func (h *IntegrationHandler) CancelCampaign(c *fiber.Ctx) error {
	companyID, err := strconv.ParseInt(c.Query("company_id"), 10, 64)
	if err != nil || companyID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid company_id"})
	}

	record, err := h.campaignService.CancelScheduled(c.UserContext(), authdomain.Actor{
		CompanyID: companyID,
		Role:      authdomain.RoleOperator,
	}, c.Params("id"))
	if err != nil {
		return campaignError(c, err)
	}

	return c.JSON(fiber.Map{"data": record})
}
