package devplatform

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/pangu-sutido/mozi-builder/mozi"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests for the visual development platform.
type Handler struct {
	svc *Service
}

// NewHandler creates a new dev platform handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ============================================================================
// Model management
// ============================================================================

// ListModels returns all modules and models.
func (h *Handler) ListModels(c *gin.Context) {
	modules, err := h.svc.ListModules(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, modules)
}

// CreateModule creates a module.
func (h *Handler) CreateModule(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body: " + err.Error()})
		return
	}
	mod, err := decodeModuleJSON(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	created, err := h.svc.CreateModule(c.Request.Context(), mod)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, encodeModuleSummary(created))
}

// UpdateModule updates module metadata.
func (h *Handler) UpdateModule(c *gin.Context) {
	name := c.Param("module")
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body: " + err.Error()})
		return
	}
	mod, err := decodeModuleJSON(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updated, err := h.svc.UpdateModule(c.Request.Context(), name, mod)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, encodeModuleSummary(updated))
}

// DeleteModule deletes an empty module.
func (h *Handler) DeleteModule(c *gin.Context) {
	name := c.Param("module")
	if err := h.svc.DeleteModule(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// GetModel returns a single model detail.
func (h *Handler) GetModel(c *gin.Context) {
	name := c.Param("name")
	model, err := h.svc.GetModel(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, model)
}

// CreateModel creates a new model from YAML.
func (h *Handler) CreateModel(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body: " + err.Error()})
		return
	}

	var model *mozi.ModelIR
	if isJSONRequest(c) {
		payload, err := decodeModelJSON(body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		model, err = h.svc.CreateModelIR(c.Request.Context(), payload)
	} else {
		model, err = h.svc.CreateModel(c.Request.Context(), string(body))
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, model)
}

// UpdateModel updates an existing model.
func (h *Handler) UpdateModel(c *gin.Context) {
	name := c.Param("name")
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body: " + err.Error()})
		return
	}

	var model *mozi.ModelIR
	if isJSONRequest(c) {
		payload, err := decodeModelJSON(body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		model, err = h.svc.UpdateModelIR(c.Request.Context(), name, payload)
	} else {
		model, err = h.svc.UpdateModel(c.Request.Context(), name, string(body))
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, model)
}

// DeleteModel deletes a model.
func (h *Handler) DeleteModel(c *gin.Context) {
	name := c.Param("name")
	if err := h.svc.DeleteModel(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// ============================================================================
// ER Diagram
// ============================================================================

// ERDiagram returns Mermaid ER DSL.
// Optional query parameter: ?module=xxx to filter by module.
func (h *Handler) ERDiagram(c *gin.Context) {
	module := c.Query("module")
	dsl, err := h.svc.ERDiagram(c.Request.Context(), module)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.String(http.StatusOK, dsl)
}

// ============================================================================
// Validation
// ============================================================================

// Validate validates a model.
func (h *Handler) Validate(c *gin.Context) {
	name := c.Param("name")
	result, err := h.svc.ValidateModel(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// ============================================================================
// Diff
// ============================================================================

// GetDiff returns a structured diff for a model.
func (h *Handler) GetDiff(c *gin.Context) {
	name := c.Param("name")
	result, err := h.svc.GetDiff(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetChangePlan returns an AI Coding task contract for a model change.
func (h *Handler) GetChangePlan(c *gin.Context) {
	name := c.Param("name")
	result, err := h.svc.ChangePlan(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// SyncModel records the current model version in the manifest, marking it as
// synced to code.
func (h *Handler) SyncModel(c *gin.Context) {
	module := c.Param("module")
	name := c.Param("name")
	if err := h.svc.SyncModel(c.Request.Context(), module, name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "synced", "model_ref": module + "/" + name})
}

// ListAPIAssets returns an OpenAPI-derived API asset index for the workbench.
func (h *Handler) ListAPIAssets(c *gin.Context) {
	result, err := h.svc.ListAPIAssets(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// SaveAPIEndpointOverride updates lightweight curation metadata for one endpoint.
func (h *Handler) SaveAPIEndpointOverride(c *gin.Context) {
	var input APIEndpointOverrideInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.EndpointID == "" {
		input.EndpointID = c.Param("id")
	}
	if err := h.svc.SaveAPIEndpointOverride(c.Request.Context(), input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "saved", "endpoint_id": input.EndpointID})
}

// ListDesignDictionaryItems returns business-maintained dictionary options.
func (h *Handler) ListDesignDictionaryItems(c *gin.Context) {
	dictionaryID := c.Param("dictionary")
	includeDisabled := c.Query("include_disabled") == "true"
	result, err := h.svc.ListDesignDictionaryItems(c.Request.Context(), dictionaryID, includeDisabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// SaveDesignDictionaryItem creates or updates one dictionary option.
func (h *Handler) SaveDesignDictionaryItem(c *gin.Context) {
	dictionaryID := c.Param("dictionary")
	var input DesignDictionaryItemInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.Value == "" {
		input.Value = c.Param("value")
	}
	if err := h.svc.SaveDesignDictionaryItem(c.Request.Context(), dictionaryID, input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "saved", "dictionary_id": dictionaryID, "value": input.Value})
}

// DeleteDesignDictionaryItem deletes one dictionary option.
func (h *Handler) DeleteDesignDictionaryItem(c *gin.Context) {
	dictionaryID := c.Param("dictionary")
	value := c.Param("value")
	if err := h.svc.DeleteDesignDictionaryItem(c.Request.Context(), dictionaryID, value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted", "dictionary_id": dictionaryID, "value": value})
}

func isJSONRequest(c *gin.Context) bool {
	return strings.Contains(c.GetHeader("Content-Type"), "application/json")
}

func decodeModelJSON(body []byte) (*mozi.ModelIR, error) {
	var model mozi.ModelIR
	if err := json.Unmarshal(body, &model); err != nil {
		return nil, err
	}

	var alias struct {
		Name  string `json:"name"`
		Model string `json:"model"`
	}
	_ = json.Unmarshal(body, &alias)
	if model.Name == "" {
		if alias.Model != "" {
			model.Name = alias.Model
		} else {
			model.Name = alias.Name
		}
	}
	return &model, nil
}

func decodeModuleJSON(body []byte) (*mozi.ModuleIR, error) {
	var mod mozi.ModuleIR
	if err := json.Unmarshal(body, &mod); err != nil {
		return nil, err
	}

	var alias struct {
		Name   string `json:"name"`
		Module string `json:"module"`
	}
	_ = json.Unmarshal(body, &alias)
	if mod.Name == "" {
		if alias.Module != "" {
			mod.Name = alias.Module
		} else {
			mod.Name = alias.Name
		}
	}
	return &mod, nil
}

func encodeModuleSummary(mod *mozi.ModuleIR) gin.H {
	return gin.H{
		"name":        mod.Name,
		"label":       mod.Label,
		"description": mod.Description,
		"icon":        mod.Icon,
		"api_prefix":  mod.APIPrefix,
		"model_count": len(mod.Models),
		"models":      []gin.H{},
	}
}
