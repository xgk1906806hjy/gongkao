package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"gongkao/internal/config"
	"gongkao/internal/crawler"
	"gongkao/internal/pkg"
	"gongkao/internal/service"
)

func main() {
	config.LoadDotEnv(".env", ".env.example")
	cfg := config.Load()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	db, err := pkg.OpenDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	if err := pkg.AutoMigrate(db); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	items, err := crawler.CollectQuestions(ctx, db, cfg)
	if err != nil {
		log.Fatal(err)
	}
	path, err := crawler.SaveQuestionBankJSON(cfg, items)
	if err != nil {
		log.Fatal(err)
	}

	prac := service.NewPracticeService(db)
	for _, c := range []struct {
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
	} {
		if _, err := prac.EnsureCategory(c.Name, c.Sort); err != nil {
			log.Fatal(err)
		}
	}

	n, err := prac.ImportQuestionsSkipDup(items)
	if err != nil {
		log.Fatal(err)
	}
	total, _ := prac.CountQuestions()
	list, _ := prac.ListCategories()
	fmt.Printf("collected=%d imported_new=%d total_questions=%d categories=%d path=%s\n",
		len(items), n, total, len(list), path)
	for _, c := range list {
		fmt.Printf("  - %s (id=%d)\n", c.Name, c.ID)
	}
}
