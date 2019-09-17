package main

import (
	"fmt"
	"math"
	"html/template"
	//"io/ioutil"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/http"
	//"regexp"
)

var templates = template.Must(template.ParseFiles("index.html"))

func get_revisions(db *sql.DB) (vec []int) {
	rows, err := db.Query("SELECT DISTINCT revision FROM processed_csb WHERE revision >= 800000 ORDER BY revision")
	if err != nil {
		log.Fatal(err)
	}
	for rows.Next() {
		var revision int
		err = rows.Scan(&revision)
		if err != nil {
			log.Fatal(err)
		}
		vec = append(vec, revision)
	}
	return
}

type ComparisonCSB struct {
	Config_file string
	Time_a      float64
	Time_b      float64
	Memory_a    float64
	Memory_b    float64
}

func to_rel_change_string(a float64, b float64) string {
	rel_change := b/a - 1.0
	if math.IsNaN(rel_change) || rel_change == -1.0 {
		return "?"
	} else if rel_change > 0.0 {
		return fmt.Sprintf("+%.1f%%", rel_change * 100)
	} else {
		return fmt.Sprintf("%.1f%%", rel_change * 100)
	}
}

func rel_change_to_color_string(a float64, b float64) string {
	rel_change := b/a - 1.0
	if math.IsNaN(rel_change) || rel_change == -1.0 || rel_change > 0.05 {
		return "#f00"
	} else if rel_change < -0.05 {
		return "#0a0"
	} else {
		return "#000"
	}
}

func comparison_csb(db *sql.DB, r1 int, r2 int) (res []ComparisonCSB) {
	stmt, err := db.Prepare("SELECT a.config_file, " +
		"AVG(a.player_total_time), AVG(b.player_total_time), " +
		"AVG(a.memory_peak), AVG(b.memory_peak) " +
		"FROM processed_csb b " +
		"LEFT JOIN processed_csb a ON a.config_file = b.config_file " +
		"WHERE a.revision=? AND b.revision=? GROUP BY a.config_file ")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()
	rows, err := stmt.Query(r1, r2)
	if err != nil {
		log.Fatal(err)
	}
	for rows.Next() {
		var row ComparisonCSB
		err = rows.Scan(&row.Config_file, &row.Time_a, &row.Time_b, &row.Memory_a, &row.Memory_b)
		if err != nil {
			log.Fatal(err)
		}
		res = append(res, row)
		//fmt.Println(row.config_file, row.time_a, row.time_b, row.memory_a, row.memory_b)
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}
	return
}

type IndexPage struct {
	Title         string
	Revision_low  int
	Revision_high int
	Revisions     []int
	Csb_rows      []ComparisonCSB
	ToRelative    func(float64, float64) string
	ToColor       func(float64, float64) string
}

func index(w http.ResponseWriter, r *http.Request) {
	revision_low, revision_high := 897000, 897500
	db, err := sql.Open("sqlite3", "/Users/fabian/Dev/Rust/bench/cutsim-testreport.db")
	if err != nil {
		log.Fatal(err)
	}
	revisions := get_revisions(db)
	csb_rows := comparison_csb(db, revision_low, revision_high)
	/*for _, r := range get_revisions(db) {
		fmt.Fprintf(w, "%d<br>", r)
	}
	for _, row := range comparison_csb(db, 897000, 897500) {
		fmt.Fprintf(w, "%s %f %f %f %f", row.config_file, row.time_a, row.time_b, row.memory_a, row.memory_b)
	}*/
	p := IndexPage{
		Title:         "CutSim benchmarks",
		Revision_low:  revision_low,
		Revision_high: revision_high,
		Revisions:     revisions,
		Csb_rows:      csb_rows,
		ToRelative:    to_rel_change_string,
		ToColor:       rel_change_to_color_string}
	err = templates.ExecuteTemplate(w, "index.html", p)
	if err != nil {
		fmt.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	http.HandleFunc("/", index)

	log.Fatal(http.ListenAndServe(":8000", nil))
}
