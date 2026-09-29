package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter retorna um middleware HTTP que limita as requisições baseadas em IP.
// r: taxa de requisições por segundo permitida.
// b: "burst" máximo de requisições instantâneas permitidas.
func RateLimiter(r rate.Limit, b int) func(http.Handler) http.Handler {
	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)

	// Goroutine de limpeza para remover IPs inativos e evitar vazamento de memória.
	go func() {
		for {
			time.Sleep(time.Minute)
			mu.Lock()
			for ip, c := range clients {
				if time.Since(c.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// Extrai IP real, lidando com portas e X-Forwarded-For
			ip, _, err := net.SplitHostPort(req.RemoteAddr)
			if err != nil {
				ip = req.RemoteAddr
			}

			if fwd := req.Header.Get("X-Forwarded-For"); fwd != "" {
				// X-Forwarded-For pode conter uma lista de IPs, pegamos o primeiro
				ips := strings.Split(fwd, ",")
				ip = strings.TrimSpace(ips[0])
			}

			mu.Lock()
			if _, found := clients[ip]; !found {
				clients[ip] = &client{limiter: rate.NewLimiter(r, b)}
			}
			clients[ip].lastSeen = time.Now()

			if !clients[ip].limiter.Allow() {
				mu.Unlock()
				http.Error(w, "Muitas requisições. Tente novamente mais tarde.", http.StatusTooManyRequests)
				return
			}
			mu.Unlock()

			next.ServeHTTP(w, req)
		})
	}
}
