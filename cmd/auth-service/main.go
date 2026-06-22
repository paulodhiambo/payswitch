package main

import (
	"crypto/rand"
	"encoding/hex"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// User represents credentials and claims matching the gateway and portal requirements.
type User struct {
	Username      string
	Password      string
	Email         string
	Name          string
	Role          string
	ParticipantID string
}

// Pre-defined dev users replicating the Authentik configurations.
var users = map[string]User{
	"switch-admin": {
		Username:      "switch-admin",
		Password:      "switchadmin",
		Email:         "switch-admin@switch.local",
		Name:          "Switch Admin",
		Role:          "SWITCH_ADMIN",
		ParticipantID: "",
	},
	"switch-ops": {
		Username:      "switch-ops",
		Password:      "switchops",
		Email:         "switch-ops@switch.local",
		Name:          "Switch Ops",
		Role:          "SWITCH_OPS",
		ParticipantID: "",
	},
	"bank-admin": {
		Username:      "bank-admin",
		Password:      "bankadmin",
		Email:         "bank-admin@switch.local",
		Name:          "Bank A Admin",
		Role:          "BANK_ADMIN",
		ParticipantID: "BANKABICX",
	},
	"bank-viewer": {
		Username:      "bank-viewer",
		Password:      "bankviewer",
		Email:         "bank-viewer@switch.local",
		Name:          "Bank A Viewer",
		Role:          "BANK_VIEWER",
		ParticipantID: "BANKABICX",
	},
}

// Thread-safe in-memory session store.
var (
	sessions   = make(map[string]User)
	sessionsMu sync.RWMutex
)

func createSession(user User) string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Printf("Error generating session token: %v", err)
	}
	token := hex.EncodeToString(b)

	sessionsMu.Lock()
	sessions[token] = user
	sessionsMu.Unlock()
	return token
}

func getSession(token string) (User, bool) {
	sessionsMu.RLock()
	user, exists := sessions[token]
	sessionsMu.RUnlock()
	return user, exists
}

func deleteSession(token string) {
	sessionsMu.Lock()
	delete(sessions, token)
	sessionsMu.Unlock()
}

func findUser(loginID string) (User, bool) {
	for _, u := range users {
		if strings.EqualFold(u.Username, loginID) || strings.EqualFold(u.Email, loginID) {
			return u, true
		}
	}
	return User{}, false
}

type loginPageData struct {
	Error    string
	Redirect string
	Username string
}

const loginHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Paysys Switch - Login</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;500;600;700;800&family=JetBrains+Mono:wght@400;500;700&display=swap" rel="stylesheet">
    <script>
        tailwind.config = {
            theme: {
                extend: {
                    fontFamily: {
                        sans: ['Outfit', 'sans-serif'],
                        mono: ['JetBrains Mono', 'monospace'],
                    }
                }
            }
        }
    </script>
    <style>
        body {
            background-color: #06080e;
        }
        .glow-indigo {
            box-shadow: 0 0 25px rgba(99, 102, 241, 0.2);
        }
        .glass-panel {
            background: rgba(13, 18, 30, 0.45);
            backdrop-filter: blur(16px);
            -webkit-backdrop-filter: blur(16px);
            border: 1px solid rgba(255, 255, 255, 0.05);
        }
        .anim-pulse {
            animation: pulse 6s cubic-bezier(0.4, 0, 0.6, 1) infinite;
        }
        @keyframes pulse {
            0%, 100% { opacity: 0.1; transform: scale(1); }
            50% { opacity: 0.2; transform: scale(1.05); }
        }
    </style>
</head>
<body class="min-h-screen flex items-center justify-center relative overflow-hidden font-sans text-gray-200">
    <!-- Background glowing mesh blobs -->
    <div class="absolute top-[-20%] left-[-10%] w-[60%] h-[60%] bg-indigo-900/10 rounded-full blur-[120px] pointer-events-none anim-pulse"></div>
    <div class="absolute bottom-[-20%] right-[-10%] w-[60%] h-[60%] bg-violet-900/15 rounded-full blur-[160px] pointer-events-none anim-pulse" style="animation-delay: 3s;"></div>

    <div class="w-full max-w-lg p-6 relative z-10">
        <!-- Logo Header -->
        <div class="flex items-center justify-center gap-3 mb-8">
            <div class="bg-gradient-to-br from-indigo-500 to-violet-600 p-2.5 rounded-xl shadow-lg glow-indigo">
                <svg class="h-6 w-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                </svg>
            </div>
            <div class="text-left">
                <span class="font-extrabold text-xl tracking-wider bg-gradient-to-r from-indigo-200 via-slate-100 to-violet-200 bg-clip-text text-transparent block">PAYSYS SWITCH</span>
                <span class="text-[10px] block text-gray-500 font-mono tracking-widest uppercase">Go Native Authentication Service</span>
            </div>
        </div>

        <!-- Main Card -->
        <div class="glass-panel p-8 rounded-2xl shadow-2xl relative overflow-hidden">
            <div class="absolute top-0 left-0 w-full h-[3px] bg-gradient-to-r from-indigo-500 via-purple-500 to-pink-500"></div>

            <div class="text-center mb-6">
                <h2 class="text-xl font-bold text-white tracking-wide">Sign in to Participant Console</h2>
                <p class="text-xs text-gray-400 mt-1">Replacing Authentik with direct secure login</p>
            </div>

            <!-- Error Banner -->
            {{if .Error}}
            <div class="bg-rose-950/40 border border-rose-900/40 rounded-xl p-3 flex gap-2 items-start text-xs text-rose-300 mb-6 transition-all duration-300">
                <svg class="h-4 w-4 text-rose-400 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
                <p>{{.Error}}</p>
            </div>
            {{end}}

            <!-- Form -->
            <form action="/login?rd={{.Redirect}}" method="POST" class="space-y-4">
                <div class="space-y-1.5">
                    <label class="text-xs font-mono text-gray-400 block">Username or Email</label>
                    <input
                        type="text"
                        name="username"
                        required
                        placeholder="switch-admin or email@domain.com"
                        value="{{.Username}}"
                        class="w-full bg-[#070a12] border border-slate-800 rounded-xl px-4 py-2.5 text-sm text-gray-200 placeholder-gray-500 focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-all font-sans"
                    />
                </div>

                <div class="space-y-1.5">
                    <label class="text-xs font-mono text-gray-400 block">Password</label>
                    <input
                        type="password"
                        name="password"
                        required
                        placeholder="••••••••••••"
                        class="w-full bg-[#070a12] border border-slate-800 rounded-xl px-4 py-2.5 text-sm text-gray-200 placeholder-gray-500 focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-all font-mono"
                    />
                </div>

                <button
                    type="submit"
                    class="w-full bg-gradient-to-r from-indigo-600 to-violet-600 hover:from-indigo-500 hover:to-violet-500 text-white font-bold text-sm py-2.5 rounded-xl shadow-lg hover:shadow-indigo-500/20 active:scale-[0.98] transition-all flex items-center justify-center gap-2 mt-6 cursor-pointer"
                >
                    <span>Sign In</span>
                    <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M14 5l7 7m0 0l-7 7m7-7H3" />
                    </svg>
                </button>
            </form>
        </div>

        <!-- Available Credentials Helper Card -->
        <div class="glass-panel mt-6 p-5 rounded-2xl border-slate-800/80 shadow-lg">
            <h3 class="text-xs font-bold text-gray-300 mb-3 flex items-center gap-2">
                <svg class="h-4 w-4 text-indigo-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                Sandbox Demo Accounts
            </h3>
            <div class="space-y-2 text-xs font-mono">
                <div class="flex justify-between items-center py-1 border-b border-slate-800/60">
                    <span class="text-gray-400">switch-admin <span class="text-[10px] text-gray-600">(Admin)</span></span>
                    <span class="text-indigo-300">switchadmin</span>
                </div>
                <div class="flex justify-between items-center py-1 border-b border-slate-800/60">
                    <span class="text-gray-400">switch-ops <span class="text-[10px] text-gray-600">(Ops)</span></span>
                    <span class="text-indigo-300">switchops</span>
                </div>
                <div class="flex justify-between items-center py-1 border-b border-slate-800/60">
                    <span class="text-gray-400">bank-admin <span class="text-[10px] text-gray-600">(Bank A Admin)</span></span>
                    <span class="text-indigo-300">bankadmin</span>
                </div>
                <div class="flex justify-between items-center py-1">
                    <span class="text-gray-400">bank-viewer <span class="text-[10px] text-gray-600">(Bank A Viewer)</span></span>
                    <span class="text-indigo-300">bankviewer</span>
                </div>
            </div>
        </div>

        <p class="text-center text-[10px] text-gray-500 mt-6 font-mono">
            Secure Session Store &bull; Port 9000 &bull; Go Auth Service
        </p>
    </div>
</body>
</html>`

var tmpl = template.Must(template.New("login").Parse(loginHTML))

func handleAuth(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("switch_session")
	if err == nil && cookie.Value != "" {
		if user, ok := getSession(cookie.Value); ok {
			// User is authenticated! Set the headers Kong expects.
			w.Header().Set("X-authentik-uid", user.Username)
			w.Header().Set("X-authentik-username", user.Username)
			w.Header().Set("X-authentik-name", user.Name)
			w.Header().Set("X-authentik-email", user.Email)
			w.Header().Set("X-authentik-groups", "admins")
			w.Header().Set("X-User-Role", user.Role)
			w.Header().Set("X-Participant-Id", user.ParticipantID)
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	// Not authenticated. Find the original path requested.
	// Traefik forwardAuth sets X-Forwarded-Uri with the original request path.
	origURI := r.Header.Get("X-Forwarded-Uri")
	if origURI == "" {
		origURI = r.Header.Get("X-Original-URI")
	}
	if origURI == "" {
		origURI = r.Header.Get("X-Original-Url")
	}
	if origURI == "" {
		origURI = "/portal/"
	}

	// If the request is an API request, return 401 directly
	if strings.Contains(origURI, "/api/") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"unauthorized"}`))
		return
	}

	// Construct an absolute login URL so Traefik doesn't resolve it against
	// the internal auth-service URL when forwarding the redirect to the browser.
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "http"
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	loginURL := proto + "://" + host + "/login"
	rdUrl := loginURL
	if origURI != "" && origURI != "/login" && origURI != "/logout" {
		rdUrl = loginURL + "?rd=" + url.QueryEscape(origURI)
	}
	w.Header().Set("Location", rdUrl)
	w.WriteHeader(http.StatusFound)
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		rd := r.URL.Query().Get("rd")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tmpl.Execute(w, loginPageData{Redirect: rd})
		return
	}

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_ = tmpl.Execute(w, loginPageData{Error: "Invalid form submission"})
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")
		rd := r.URL.Query().Get("rd")
		if rd == "" {
			rd = r.FormValue("rd")
		}

		user, ok := findUser(username)
		if !ok || user.Password != password {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_ = tmpl.Execute(w, loginPageData{
				Error:    "Invalid username/email or password.",
				Username: username,
				Redirect: rd,
			})
			return
		}

		// Success: create session cookie
		token := createSession(user)
		http.SetCookie(w, &http.Cookie{
			Name:     "switch_session",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			MaxAge:   86400, // 24 hours
		})

		if rd == "" {
			rd = "/portal/"
		}
		w.Header().Set("Location", rd)
		w.WriteHeader(http.StatusFound)
	}
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("switch_session")
	if err == nil {
		deleteSession(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "switch_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	w.Header().Set("Location", "/login")
	w.WriteHeader(http.StatusFound)
}

func main() {
	http.HandleFunc("/auth", handleAuth)
	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/logout", handleLogout)

	log.Println("Auth Service starting on :9000...")
	if err := http.ListenAndServe(":9000", nil); err != nil {
		log.Fatalf("Auth Service failed: %v", err)
	}
}
