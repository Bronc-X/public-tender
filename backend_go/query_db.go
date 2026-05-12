//go:build ignore

package main

import (
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type Qualification struct {
	ID                 string `db:"id"`
	QualificationName  string `db:"qualification_name"`
	QualificationLevel string `db:"qualification_level"`
	QualificationType  string `db:"qualification_type"`
	CompanyID          string `db:"company_id"`
}

func main() {
	db, err := sqlx.Open("mysql", "root:@tcp(localhost:3306)/bid_data?parseTime=true")
	if err != nil {
		log.Fatal(err)
	}

	var quals []Qualification
	err = db.Select(&quals, "SELECT id, qualification_name, qualification_level, qualification_type, company_id FROM qualification LIMIT 10")
	if err != nil {
		log.Fatal(err)
	}

	for _, q := range quals {
		fmt.Printf("ID: %s, Name: %s, Level: %s, Type: %s, CompanyID: %s\n", q.ID, q.QualificationName, q.QualificationLevel, q.QualificationType, q.CompanyID)
	}
}
