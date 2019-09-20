package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
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
	ConfigFile string
	TimeA      float64
	TimeB      float64
	MemoryA    float64
	MemoryB    float64
}

func comparison_csb(db *sql.DB, r1 int, r2 int, sort_by string) (res []ComparisonCSB) {
	stmt, err := db.Prepare("SELECT a.config_file, " +
		"AVG(a.player_total_time), AVG(b.player_total_time), " +
		"AVG(a.memory_peak), AVG(b.memory_peak) " +
		"FROM processed_csb b " +
		"INNER JOIN processed_csb a ON a.config_file = b.config_file " +
		"WHERE a.revision=? AND b.revision=? GROUP BY a.config_file " +
		"ORDER BY " + sort_by)
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
		err = rows.Scan(&row.ConfigFile, &row.TimeA, &row.TimeB, &row.MemoryA, &row.MemoryB)
		if err != nil {
			log.Fatal(err)
		}
		row.ConfigFile = strings.Replace(row.ConfigFile, `D:\Jenkins\checkouts\trunk\QA_new\testcases\`, "", 1)
		res = append(res, row)
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}
	return
}

type ComparisonIni struct {
	ConfigFile string
	CutTimeA   float64
	CutTimeB   float64
	DrawTimeA  float64
	DrawTimeB  float64
	MemoryA    float64
	MemoryB    float64
}

func comparison_ini(db *sql.DB, r1 int, r2 int, sort_by string) (res []ComparisonIni) {
	stmt, err := db.Prepare("SELECT a.config_file, " +
		"AVG(a.cutting_time), AVG(b.cutting_time), " +
		"AVG(a.draw_time), AVG(b.draw_time), " +
		"AVG(a.memory_peak), AVG(b.memory_peak) " +
		"FROM processed_ini b " +
		"INNER JOIN processed_ini a ON a.config_file = b.config_file " +
		"WHERE a.revision=? AND b.revision=? GROUP BY a.config_file " +
		"ORDER BY " + sort_by)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()
	rows, err := stmt.Query(r1, r2)
	if err != nil {
		log.Fatal(err)
	}
	for rows.Next() {
		var row ComparisonIni
		err = rows.Scan(
			&row.ConfigFile,
			&row.CutTimeA, &row.CutTimeB,
			&row.DrawTimeA, &row.DrawTimeB,
			&row.MemoryA, &row.MemoryB)
		if err != nil {
			log.Fatal(err)
		}
		row.ConfigFile = strings.Replace(row.ConfigFile, `D:\Jenkins\checkouts\trunk\QA_new\testcases\`, "", 1)
		res = append(res, row)
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}
	return
}

type IndexPage struct {
	Title        string
	RevisionLow  int
	RevisionHigh int
	SortBy       string
	Revisions    []int
	CsbRows      []ComparisonCSB
	IniRows      []ComparisonIni
	ToRelative   func(float64, float64) string
	ToColor      func(float64, float64) string
}

func index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "404 file not found", http.StatusNotFound)
		return
	}
	db, err := sql.Open("sqlite3", "/Users/fabian/Dev/Rust/bench/cutsim-testreport.db")
	if err != nil {
		log.Fatal(err)
	}
	revisions := get_revisions(db)

	revision_low, err := strconv.Atoi(r.URL.Query().Get("r1"))
	if err != nil {
		revision_low = revisions[len(revisions)-6]
	}
	revision_high, err := strconv.Atoi(r.URL.Query().Get("r2"))
	if err != nil {
		revision_high = revisions[len(revisions)-1]
	}
	sort_by := r.URL.Query().Get("sort")
	if sort_by == "" {
		sort_by = "name"
	}

	ini_sort_by := "a.config_file"
	csb_sort_by := "a.config_file"
	switch sort_by {
	case "cut time":
		ini_sort_by = "AVG(a.cutting_time) / AVG(b.cutting_time)"
		csb_sort_by = "AVG(a.player_total_time) / AVG(b.player_total_time)"
	case "draw time":
		ini_sort_by = "AVG(a.cutting_time) / AVG(b.cutting_time)"
		csb_sort_by = "AVG(a.player_total_time) / AVG(b.player_total_time)"
	case "memory":
		ini_sort_by = "AVG(a.memory_peak) / AVG(b.memory_peak)"
		csb_sort_by = "AVG(a.memory_peak) / AVG(b.memory_peak)"
	}

	csb_rows := comparison_csb(db, revision_low, revision_high, csb_sort_by)
	ini_rows := comparison_ini(db, revision_low, revision_high, ini_sort_by)
	p := IndexPage{
		Title:        "CutSim benchmarks",
		RevisionLow:  revision_low,
		RevisionHigh: revision_high,
		SortBy:       sort_by,
		Revisions:    revisions,
		CsbRows:      csb_rows,
		IniRows:      ini_rows,
		ToRelative:   to_rel_change_string,
		ToColor:      rel_change_to_color_string}
	err = templates.ExecuteTemplate(w, "index.html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
	}
}

func to_rel_change_string(a float64, b float64) string {
	rel_change := b/a - 1.0
	if math.IsNaN(rel_change) || rel_change == -1.0 {
		return "?"
	} else if rel_change > 0.0 {
		return fmt.Sprintf("+%.1f%%", rel_change*100)
	} else {
		return fmt.Sprintf("%.1f%%", rel_change*100)
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

type Row struct {
	X int     `json:"x"`
	Y float64 `json:"y"`
	V float64 `json:"v"`
}
type Dataset struct {
	Label           string `json:"label"`
	BackgroundColor string `json:"backgroundColor"`
	BorderColor     string `json:"borderColor"`
	Fill            bool   `json:"fill"`
	Data            []Row  `json:"data"`
}
type Response struct {
	Labels   []int     `json:"labels"`
	Datasets []Dataset `json:"datasets"`
}

func api_csb_file_graph_json(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "/Users/fabian/Dev/Rust/bench/cutsim-testreport.db")
	if err != nil {
		log.Fatal(err)
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id parameter missing", http.StatusBadRequest)
		return
	}

	rows, err := db.Query(
		"SELECT revision, AVG(memory_peak), AVG(player_total_time) "+
			"FROM processed_csb WHERE config_file LIKE ?1 "+
			"AND revision >= 800000 GROUP BY revision ORDER BY revision", id)
	if err != nil {
		log.Fatal(err)
	}

	revisions := []int{}
	first_memory_peak := 0.0
	first_player_total_time := 0.0
	memory_rows := []Row{}
	player_total_time_rows := []Row{}
	for rows.Next() {
		var revision int
		var memory_peak float64
		var player_total_time float64
		err = rows.Scan(&revision, &memory_peak, &player_total_time)
		if err != nil {
			log.Fatal(err)
		}
		revisions = append(revisions, revision)
		if first_memory_peak == 0.0 {
			first_memory_peak = memory_peak
		}
		if first_player_total_time == 0.0 {
			first_player_total_time = player_total_time
		}
		if first_memory_peak == 0.0 || first_player_total_time == 0.0 {
			continue
		}
		memory_rows = append(memory_rows, Row{
			X: revision,
			Y: (memory_peak / first_memory_peak),
			V: memory_peak,
		})
		player_total_time_rows = append(player_total_time_rows, Row{
			X: revision,
			Y: player_total_time / first_player_total_time,
			V: player_total_time,
		})
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}

	response := Response{
		Labels: revisions,
		Datasets: []Dataset{
			Dataset{
				Label:           "Memory",
				BackgroundColor: "rgb(54, 162, 235)",
				BorderColor:     "rgb(54, 162, 235)",
				Fill:            false,
				Data:            memory_rows,
			},
			Dataset{
				Label:           "Run Time",
				BackgroundColor: "rgb(255, 159, 64)",
				BorderColor:     "rgb(255, 159, 64)",
				Fill:            false,
				Data:            player_total_time_rows,
			},
		},
	}

	response_json, err := json.Marshal(response)
	if err != nil {
		log.Fatal(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(response_json)
}

func api_ini_file_graph_json(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "/Users/fabian/Dev/Rust/bench/cutsim-testreport.db")
	if err != nil {
		log.Fatal(err)
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id parameter missing", http.StatusBadRequest)
		return
	}

	rows, err := db.Query(
		"SELECT revision, AVG(memory_peak), AVG(cutting_time), AVG(draw_time) "+
			"FROM processed_ini WHERE config_file LIKE ?1 "+
			"AND revision >= 800000 GROUP BY revision ORDER BY revision", id)
	if err != nil {
		log.Fatal(err)
	}

	revisions := []int{}
	first_memory_peak := 0.0
	first_cutting_time := 0.0
	first_draw_time := 0.0
	memory_rows := []Row{}
	cutting_time_rows := []Row{}
	draw_time_rows := []Row{}
	for rows.Next() {
		var revision int
		var memory_peak float64
		var cutting_time float64
		var draw_time float64
		err = rows.Scan(&revision, &memory_peak, &cutting_time, &draw_time)
		if err != nil {
			log.Fatal(err)
		}
		revisions = append(revisions, revision)
		if first_memory_peak == 0.0 {
			first_memory_peak = memory_peak
		}
		if first_cutting_time == 0.0 {
			first_cutting_time = cutting_time
		}
		if first_draw_time == 0.0 {
			first_draw_time = draw_time
		}
		if first_memory_peak == 0.0 || first_cutting_time == 0.0 || first_draw_time == 0 {
			continue
		}
		memory_rows = append(memory_rows, Row{
			X: revision,
			Y: (memory_peak / first_memory_peak),
			V: memory_peak,
		})
		cutting_time_rows = append(cutting_time_rows, Row{
			X: revision,
			Y: cutting_time / first_cutting_time,
			V: cutting_time,
		})
		draw_time_rows = append(draw_time_rows, Row{
			X: revision,
			Y: draw_time / first_draw_time,
			V: draw_time,
		})
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}

	response := Response{
		Labels: revisions,
		Datasets: []Dataset{
			Dataset{
				Label:           "Memory",
				BackgroundColor: "rgb(54, 162, 235)",
				BorderColor:     "rgb(54, 162, 235)",
				Fill:            false,
				Data:            memory_rows,
			},
			Dataset{
				Label:           "Cutting Time",
				BackgroundColor: "rgb(255, 159, 64)",
				BorderColor:     "rgb(255, 159, 64)",
				Fill:            false,
				Data:            cutting_time_rows,
			},
			Dataset{
				Label:           "Draw Time",
				BackgroundColor: "rgb(75, 192, 192)",
				BorderColor:     "rgb(75, 192, 192)",
				Fill:            false,
				Data:            draw_time_rows,
			},
		},
	}

	response_json, err := json.Marshal(response)
	if err != nil {
		log.Fatal(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(response_json)
}

func main() {
	http.HandleFunc("/", index)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	http.HandleFunc("/api/file/ini", api_ini_file_graph_json)
	http.HandleFunc("/api/file/csb", api_csb_file_graph_json)

	log.Fatal(http.ListenAndServe(":8000", nil))
}
