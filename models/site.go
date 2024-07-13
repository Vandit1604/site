package models

import (
	"database/sql"
	"log"

	"github.com/vandit1604/site/utils"
)

var db *sql.DB

func InitDB(dataSourceName string) error {
	var err error
	db, err = sql.Open("postgres", dataSourceName)
	if err != nil {
		return err
	}

	return db.Ping()
}

type Article struct {
	ID      int64  `json:"id"`
	Title   string `json:"string"`
	Content string `json:"content"`
	// figure out how to get current date. having problems with the struct
	Date string `json:"date"`
}

func RegisterArticle(at Article) error {
	_, err := db.Exec(`INSERT INTO articles (title, content, date) VALUES ($1, $2, CURRENT_TIMESTAMP)`, at.Title, at.Content)
	if err != nil {
		log.Fatalf("Error inserting article into database: %v", err)
	}
	return nil
}

func GetAllArticles() ([]Article, error) {
	rows, err := db.Query("SELECT * FROM articles")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []Article

	for rows.Next() {
		var at Article

		err := rows.Scan(&at.ID, &at.Title, &at.Content, &at.Date)
		if err != nil {
			return nil, err
		}

		at.Date = utils.FormatDate(at.Date)

		articles = append(articles, at)
	}

	return articles, nil
}

func GetArticle(ID int64) (Article, error) {
	rows, err := db.Query("SELECT * FROM articles WHERE ID=$1;", ID)
	if err != nil {
		return Article{}, err
	}
	defer rows.Close()
	var at Article
	for rows.Next() {
		err := rows.Scan(&at.ID, &at.Title, &at.Content, &at.Date)
		if err != nil {
			return Article{}, err
		}
	}

	at.Date = utils.FormatDate(at.Date)

	return at, nil
}
