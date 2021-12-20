package mediautil

import (
	"net/http"
	"time"

	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
)

// Client is the HTTP client used to fetch all images.
var Client = http.Client{
	Timeout: 4 * time.Minute,
	Transport: httpcache.NewTransport(
		diskcache.New(config.CacheDir("media")),
	),
}
