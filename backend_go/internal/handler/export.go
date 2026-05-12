package handler

import (
	"backend_go/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/xuri/excelize/v2"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

type ExportHandler struct {
	db *sqlx.DB
}

func NewExportHandler(db *sqlx.DB) *ExportHandler {
	return &ExportHandler{db: db}
}

func (h *ExportHandler) GetProjectOutputs(c *gin.Context) {
	projectID := c.Param("id")
	projectType := c.Query("type") // bid or tech

	var outputs []model.ProjectOutput
	table := "bid_project_outputs"
	if projectType == "tech" {
		table = "tech_bid_outputs"
	}

	err := h.db.Select(&outputs, "SELECT * FROM "+table+" WHERE project_id = ? ORDER BY created_at DESC", projectID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(200, outputs)
}

func (h *ExportHandler) ExportCompanyExcel(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	
	var persons []model.Person
	err := h.db.Select(&persons, "SELECT * FROM person WHERE company_id = ?", companyID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	f := excelize.NewFile()
	sheet := "Personnel"
	f.NewSheet(sheet)
	f.SetCellValue(sheet, "A1", "Name")
	f.SetCellValue(sheet, "B1", "Role")
	f.SetCellValue(sheet, "C1", "Job Status")

	for i, p := range persons {
		row := i + 2
		f.SetCellValue(sheet, "A"+strconv.Itoa(row), p.Name)
		if p.Gender != nil {
			f.SetCellValue(sheet, "B"+strconv.Itoa(row), *p.Gender)
		}
		if p.OnJobStatus != nil {
			f.SetCellValue(sheet, "C"+strconv.Itoa(row), *p.OnJobStatus)
		}
	}

	// Save to temp and serve
	tempDir := os.TempDir()
	filePath := filepath.Join(tempDir, "company_personnel.xlsx")
	if err := f.SaveAs(filePath); err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.File(filePath)
}
