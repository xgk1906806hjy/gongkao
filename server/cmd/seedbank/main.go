package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gongkao/internal/config"
	"gongkao/internal/pkg"
	"gongkao/internal/service"
)

func main() {
	file := flag.String("file", "data/question_bank/questions.json", "question bank json/csv path")
	reset := flag.Bool("reset", false, "delete existing questions/options/categories before import (dev only)")
	flag.Parse()

	config.LoadDotEnv(".env", ".env.example")
	cfg := config.Load()
	db, err := pkg.OpenDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	if err := pkg.AutoMigrate(db); err != nil {
		log.Fatal(err)
	}

	prac := service.NewPracticeService(db)
	if *reset {
		log.Println("reset: clearing practice data")
		_ = db.Exec("DELETE FROM user_answers").Error
		_ = db.Exec("DELETE FROM wrong_questions").Error
		_ = db.Exec("DELETE FROM practice_sessions").Error
		_ = db.Exec("DELETE FROM question_options").Error
		_ = db.Exec("DELETE FROM questions").Error
		_ = db.Exec("DELETE FROM question_categories").Error
	}

	abs, _ := filepath.Abs(*file)
	raw, err := os.ReadFile(abs)
	if err != nil {
		log.Fatalf("read file: %v", err)
	}

	var items []service.ImportQuestion
	ext := strings.ToLower(filepath.Ext(abs))
	switch ext {
	case ".json":
		if err := json.Unmarshal(raw, &items); err != nil {
			log.Fatalf("json: %v", err)
		}
	case ".csv":
		r := csv.NewReader(strings.NewReader(string(raw)))
		r.FieldsPerRecord = -1
		rows, err := r.ReadAll()
		if err != nil {
			log.Fatalf("csv: %v", err)
		}
		n, err := prac.ImportCSV(rows)
		if err != nil {
			log.Fatal(err)
		}
		printStats(prac, n)
		return
	default:
		log.Fatalf("unsupported file type: %s", ext)
	}

	// seed categories with sort order first
	cats := []struct {
		Name string
		Sort int
	}{
		{"行测-言语理解", 10},
		{"行测-数量关系", 20},
		{"行测-判断推理", 30},
		{"行测-资料分析", 40},
		{"公基-政治理论", 50},
		{"公基-法律基础", 60},
		{"公基-人文常识", 70},
		{"公基-科技自然", 80},
		{"时政常识", 90},
	}
	for _, c := range cats {
		if _, err := prac.EnsureCategory(c.Name, c.Sort); err != nil {
			log.Fatal(err)
		}
	}

	n, err := prac.ImportQuestionsSkipDup(items)
	if err != nil {
		log.Fatal(err)
	}
	printStats(prac, n)
}

func printStats(prac *service.PracticeService, imported int) {
	cats, _ := prac.ListCategories()
	fmt.Printf("imported=%d categories=%d\n", imported, len(cats))
	for _, c := range cats {
		fmt.Printf("  - %s (id=%d sort=%d)\n", c.Name, c.ID, c.Sort)
	}
}