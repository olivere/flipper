package server

// Blank imports ensure screen packages register their factories via init().
import (
	_ "github.com/olivere/flipper/internal/screen/demo"
	_ "github.com/olivere/flipper/internal/screen/fcbayern"
	_ "github.com/olivere/flipper/internal/screen/hackernews"
	_ "github.com/olivere/flipper/internal/screen/news"
	_ "github.com/olivere/flipper/internal/screen/static"
	_ "github.com/olivere/flipper/internal/screen/weather"
)
