package handler

import (
	"backend_go/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"fmt"
	"net/http"
	"strings"
	"time"
	"github.com/google/uuid"
	"log"
)

type PersonHandler struct {
	db *sqlx.DB
}

func NewPersonHandler(db *sqlx.DB) *PersonHandler {
	return &PersonHandler{db: db}
}

func (h *PersonHandler) ListPersons(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	persons := []model.Person{}
	err := h.db.Select(&persons, "SELECT * FROM person WHERE company_id = ? ORDER BY created_at DESC", companyID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	if len(persons) > 0 {
		var personIDs []string
		personMap := make(map[string]*model.Person)
		for i := range persons {
			personIDs = append(personIDs, persons[i].ID)
			personMap[persons[i].ID] = &persons[i]
		}

		query, args, err := sqlx.In(`
			SELECT q.*, fa.stored_path, fa.ext
			FROM qualification q 
			LEFT JOIN file_asset fa ON q.file_asset_id = fa.id 
			WHERE q.owner_type = 'person' AND q.owner_id IN (?)`, personIDs)
		if err == nil {
			query = h.db.Rebind(query)
			var allCerts []model.Qualification
			if err := h.db.Select(&allCerts, query, args...); err == nil {
				for _, cert := range allCerts {
					if cert.OwnerID != nil {
						if p, ok := personMap[*cert.OwnerID]; ok {
							if p.Certificates == nil {
								p.Certificates = []model.Qualification{}
							}
							p.Certificates = append(p.Certificates, cert)
						}
					}
				}
			}
		}
	}

	c.JSON(200, persons)
}

func (h *PersonHandler) GetPerson(c *gin.Context) {
	id := c.Param("id")
	companyID, _ := c.Get("companyID")
	var person model.Person
	err := h.db.Get(&person, "SELECT * FROM person WHERE id = ? AND company_id = ?", id, companyID)
	if err != nil {
		Error(c, http.StatusNotFound, "Person not found")
		return
	}
	var certs []model.Qualification
	q := `
		SELECT q.*, fa.stored_path, fa.ext
		FROM qualification q
		LEFT JOIN file_asset fa ON q.file_asset_id = fa.id
		WHERE q.owner_type = 'person' AND q.owner_id = ?`
	if err := h.db.Select(&certs, h.db.Rebind(q), id); err == nil && len(certs) > 0 {
		person.Certificates = certs
	}

	// Fetch proofs
	proofs := []model.PersonProof{}
	queryProofs := `
		SELECT pp.*, fa.file_name, fa.ext, fa.markdown_text 
		FROM person_proof pp
		LEFT JOIN file_asset fa ON pp.file_asset_id = fa.id
		WHERE pp.person_id = ?`
	if err := h.db.Select(&proofs, h.db.Rebind(queryProofs), id); err != nil {
		log.Printf("[Person] Error fetching proofs for %s: %v", id, err)
	}
	person.Proofs = proofs

	// Fetch related performances
	var rawPerformances []struct {
		ID                       string   `db:"id"`
		ProjectName              string   `db:"project_name"`
		ProjectManagerName       *string  `db:"project_manager_name"`
		WinningDate              *string  `db:"winning_date"`
		CompletionDate           *string  `db:"completion_date"`
		AmountValue              *float64 `db:"amount_value"`
		PmID                     *string  `db:"pm_id"`
		TechLeaderID             *string  `db:"tech_leader_id"`
		TechnicalLeaderName      *string  `db:"technical_leader_name"`
		SafetyLeaderID           *string  `db:"safety_leader_id"`
		SafetyLeaderName         *string  `db:"safety_leader_name"`
		ConstructionOfficerID    *string  `db:"construction_officer_id"`
		ConstructionOfficerName  *string  `db:"construction_officer_name"`
		QualityInspectorID       *string  `db:"quality_inspector_id"`
		QualityInspectorName     *string  `db:"quality_inspector_name"`
		DocumentationOfficerID   *string  `db:"documentation_officer_id"`
		DocumentationOfficerName *string  `db:"documentation_officer_name"`
		MaterialsOfficerID       *string  `db:"materials_officer_id"`
		MaterialsOfficerName     *string  `db:"materials_officer_name"`
		StandardsOfficerID       *string  `db:"standards_officer_id"`
		StandardsOfficerName     *string  `db:"standards_officer_name"`
		MechanicalOfficerID      *string  `db:"mechanical_officer_id"`
		MechanicalOfficerName    *string  `db:"mechanical_officer_name"`
		LaborOfficerID           *string  `db:"labor_officer_id"`
		LaborOfficerName         *string  `db:"labor_officer_name"`
	}
	perfQuery := `
		SELECT 
			id, project_name, project_manager_name, winning_date, completion_date, amount_value,
			pm_id, tech_leader_id, technical_leader_name,
			safety_leader_id, safety_leader_name,
			construction_officer_id, construction_officer_name,
			quality_inspector_id, quality_inspector_name,
			documentation_officer_id, documentation_officer_name,
			materials_officer_id, materials_officer_name,
			standards_officer_id, standards_officer_name,
			mechanical_officer_id, mechanical_officer_name,
			labor_officer_id, labor_officer_name
		FROM project_performance
		WHERE company_id = ? 
		AND (
			pm_id = ? OR project_manager_name = ? OR
			tech_leader_id = ? OR technical_leader_name = ? OR
			safety_leader_id = ? OR safety_leader_name = ? OR
			construction_officer_id = ? OR construction_officer_name = ? OR
			quality_inspector_id = ? OR quality_inspector_name = ? OR
			documentation_officer_id = ? OR documentation_officer_name = ? OR
			materials_officer_id = ? OR materials_officer_name = ? OR
			standards_officer_id = ? OR standards_officer_name = ? OR
			mechanical_officer_id = ? OR mechanical_officer_name = ? OR
			labor_officer_id = ? OR labor_officer_name = ?
		)
		ORDER BY created_at DESC
	`
	idMatch := id
	nameMatch := person.Name
	args := []interface{}{companyID}
	for i := 0; i < 10; i++ {
		args = append(args, idMatch, nameMatch)
	}

	if err := h.db.Select(&rawPerformances, h.db.Rebind(perfQuery), args...); err == nil {
		var mappedPerformances []model.PersonRelatedPerformance
		for _, raw := range rawPerformances {
			var roles []string

			isMatch := func(colID *string, colName *string) bool {
				if colID != nil && *colID == idMatch {
					return true
				}
				if colName != nil && *colName == nameMatch {
					return true
				}
				return false
			}

			if isMatch(raw.PmID, raw.ProjectManagerName) {
				roles = append(roles, "项目经理")
			}
			if isMatch(raw.TechLeaderID, raw.TechnicalLeaderName) {
				roles = append(roles, "技术负责人")
			}
			if isMatch(raw.SafetyLeaderID, raw.SafetyLeaderName) {
				roles = append(roles, "安全员")
			}
			if isMatch(raw.ConstructionOfficerID, raw.ConstructionOfficerName) {
				roles = append(roles, "施工员")
			}
			if isMatch(raw.QualityInspectorID, raw.QualityInspectorName) {
				roles = append(roles, "质量员")
			}
			if isMatch(raw.DocumentationOfficerID, raw.DocumentationOfficerName) {
				roles = append(roles, "资料员")
			}
			if isMatch(raw.MaterialsOfficerID, raw.MaterialsOfficerName) {
				roles = append(roles, "材料员")
			}
			if isMatch(raw.StandardsOfficerID, raw.StandardsOfficerName) {
				roles = append(roles, "标准员")
			}
			if isMatch(raw.MechanicalOfficerID, raw.MechanicalOfficerName) {
				roles = append(roles, "机械员")
			}
			if isMatch(raw.LaborOfficerID, raw.LaborOfficerName) {
				roles = append(roles, "劳务员")
			}

			if len(roles) > 0 {
				mappedPerformances = append(mappedPerformances, model.PersonRelatedPerformance{
					ID:                 raw.ID,
					ProjectName:        raw.ProjectName,
					RoleName:           strings.Join(roles, " / "),
					ProjectManagerName: raw.ProjectManagerName,
					WinningDate:        raw.WinningDate,
					CompletionDate:     raw.CompletionDate,
					AmountValue:        raw.AmountValue,
				})
			}
		}
		person.Performances = mappedPerformances
	} else {
		log.Printf("[Person] Error fetching performances for %s: %v", id, err)
		person.Performances = []model.PersonRelatedPerformance{}
	}

	// Fetch Educations
	if err := h.db.Select(&person.Educations, `
		SELECT id, person_id, start_date, end_date, school, degree, created_at, updated_at
		FROM person_education WHERE person_id = ? ORDER BY start_date DESC`, id); err != nil {
		log.Printf("[Person] Error fetching educations for %s: %v", id, err)
		person.Educations = []model.PersonEducation{}
	}

	// Fetch Work Experiences
	if err := h.db.Select(&person.WorkExperiences, `
		SELECT id, person_id, start_date, end_date, company, position, created_at, updated_at
		FROM person_work_experience WHERE person_id = ? ORDER BY start_date DESC`, id); err != nil {
		log.Printf("[Person] Error fetching work experiences for %s: %v", id, err)
		person.WorkExperiences = []model.PersonWorkExperience{}
	}

	c.JSON(200, person)
}

func (h *PersonHandler) CreatePersonProof(c *gin.Context) {
	id := c.Param("id")
	log.Printf("[Person] Received request to create proof for person %s", id)
	var input struct {
		FileAssetID string `json:"file_asset_id" binding:"required"`
		ProofType   string `json:"proof_type"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	proofID := uuid.New().String()
	_, err := h.db.Exec(`INSERT INTO person_proof (id, person_id, file_asset_id, proof_type) VALUES (?, ?, ?, ?)`,
		proofID, id, input.FileAssetID, input.ProofType)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(201, gin.H{"id": proofID})
}

func (h *PersonHandler) DeletePersonProof(c *gin.Context) {
	proofID := c.Param("proofID")
	_, err := h.db.Exec("DELETE FROM person_proof WHERE id = ?", proofID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func (h *PersonHandler) CreatePerson(c *gin.Context) {
	companyID, _ := c.Get("companyID")
	var input struct {
		Name                 string `json:"name" binding:"required"`
		RoleType             string `json:"role_type"`
		Specialty            string `json:"specialty"`
		CompanyName          string `json:"company_name"`
		IDNumberMasked       string `json:"id_number_masked"`
		IDCardNo             string `json:"id_card_no"`
		SocialSecurityStatus string `json:"social_security_status"`
		OnJobStatus          string `json:"on_job_status"`
		BidUsableStatus      string `json:"bid_usable_status"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var count int
	h.db.Get(&count, "SELECT COUNT(*) FROM person WHERE company_id = ?", companyID)
	id := fmt.Sprintf("%s-%d", companyID, count+1)

	query := `INSERT INTO person (id, name, role_type, specialty, company_name, id_number_masked, id_card_no, social_security_status, on_job_status, bid_usable_status, company_id) 
              VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	
	_, err := h.db.Exec(query, id, input.Name, input.RoleType, input.Specialty, input.CompanyName, input.IDNumberMasked, input.IDCardNo, input.SocialSecurityStatus, input.OnJobStatus, input.BidUsableStatus, companyID)
	if err != nil {
		// Simple retry for collision
		for i := 1; i < 100; i++ {
			id = fmt.Sprintf("%s-%d", companyID, count+1+i)
			_, err = h.db.Exec(query, id, input.Name, input.RoleType, input.Specialty, input.CompanyName, input.IDNumberMasked, input.IDCardNo, input.SocialSecurityStatus, input.OnJobStatus, input.BidUsableStatus, companyID)
			if err == nil {
				break
			}
		}
	}
	
	if err == nil {
		// --- Reverse Matching Hook ---
		// Automatically link all performances with the same name but no ID.
		// Runs synchronously for simplicity as person creation is low-volume.
		cid, _ := companyID.(string)
		go func(pID, pName, cID string) {
			// Link PM
			h.db.Exec("UPDATE project_performance SET pm_id = ? WHERE project_manager_name = ? AND company_id = ? AND (pm_id IS NULL OR pm_id = '')", pID, pName, cID)
			// Link Tech Leader
			h.db.Exec("UPDATE project_performance SET tech_leader_id = ? WHERE technical_leader_name = ? AND company_id = ? AND (tech_leader_id IS NULL OR tech_leader_id = '')", pID, pName, cID)
			// Link Safety Leader
			h.db.Exec("UPDATE project_performance SET safety_leader_id = ? WHERE safety_leader_name = ? AND company_id = ? AND (safety_leader_id IS NULL OR safety_leader_id = '')", pID, pName, cID)
		}(id, input.Name, cid)
		// -----------------------------
	}

	c.JSON(201, gin.H{"id": id})
}

func (h *PersonHandler) UpdatePerson(c *gin.Context) {
	id := c.Param("id")
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}

	forbidden := map[string]bool{"id": true, "created_at": true, "updated_at": true}
	var setClauses []string
	var args []interface{}

	for k, v := range input {
		if !forbidden[strings.ToLower(k)] {
			setClauses = append(setClauses, k+" = ?")
			args = append(args, v)
		}
	}

	if len(setClauses) == 0 {
		Error(c, http.StatusBadRequest, "no valid fields to update")
		return
	}

	args = append(args, time.Now(), id)
	query := "UPDATE person SET " + strings.Join(setClauses, ", ") + ", updated_at = ? WHERE id = ?"
	
	result, err := h.db.Exec(query, args...)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		Error(c, http.StatusNotFound, "Person not found")
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func (h *PersonHandler) DeletePerson(c *gin.Context) {
	id := c.Param("id")
	result, err := h.db.Exec("DELETE FROM person WHERE id = ?", id)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		Error(c, http.StatusNotFound, "Person not found")
		return
	}
	c.JSON(200, gin.H{"success": true})
}

// Education Handlers
func (h *PersonHandler) CreateEducation(c *gin.Context) {
	personID := c.Param("id")
	var input model.PersonEducation
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	input.ID = uuid.New().String()
	input.PersonID = personID

	_, err := h.db.NamedExec(`INSERT INTO person_education (id, person_id, start_date, end_date, school, degree) 
		VALUES (:id, :person_id, :start_date, :end_date, :school, :degree)`, &input)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(201, gin.H{"id": input.ID})
}

func (h *PersonHandler) UpdateEducation(c *gin.Context) {
	eduID := c.Param("eduID")
	var input model.PersonEducation
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	input.ID = eduID
	input.UpdatedAt = func(t time.Time) *time.Time { return &t }(time.Now())

	_, err := h.db.NamedExec(`UPDATE person_education 
		SET start_date=:start_date, end_date=:end_date, school=:school, degree=:degree, updated_at=:updated_at 
		WHERE id=:id`, &input)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func (h *PersonHandler) DeleteEducation(c *gin.Context) {
	eduID := c.Param("eduID")
	_, err := h.db.Exec("DELETE FROM person_education WHERE id = ?", eduID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, gin.H{"success": true})
}

// Work Experience Handlers
func (h *PersonHandler) CreateWorkExperience(c *gin.Context) {
	personID := c.Param("id")
	var input model.PersonWorkExperience
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	input.ID = uuid.New().String()
	input.PersonID = personID

	_, err := h.db.NamedExec(`INSERT INTO person_work_experience (id, person_id, start_date, end_date, company, position) 
		VALUES (:id, :person_id, :start_date, :end_date, :company, :position)`, &input)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(201, gin.H{"id": input.ID})
}

func (h *PersonHandler) UpdateWorkExperience(c *gin.Context) {
	workID := c.Param("workID")
	var input model.PersonWorkExperience
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, err.Error())
		return
	}
	input.ID = workID
	input.UpdatedAt = func(t time.Time) *time.Time { return &t }(time.Now())

	_, err := h.db.NamedExec(`UPDATE person_work_experience 
		SET start_date=:start_date, end_date=:end_date, company=:company, position=:position, updated_at=:updated_at 
		WHERE id=:id`, &input)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func (h *PersonHandler) DeleteWorkExperience(c *gin.Context) {
	workID := c.Param("workID")
	_, err := h.db.Exec("DELETE FROM person_work_experience WHERE id = ?", workID)
	if err != nil {
		Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, gin.H{"success": true})
}

