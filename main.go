package main

import (
	"github.com/vandit1604/site/models"
	"github.com/vandit1604/site/router"
)

func main() {
	models.ReadBlogs()
	router.Run()
}
