package caddy

type Config struct {
	Apps struct {
		HTTP struct {
			Servers map[string]Server `json:"servers"`
		} `json:"http"`
	} `json:"apps"`
}

type Server struct {
	Listen  []string `json:"listen"`
	Routes  []Route  `json:"routes"`
}

type Route struct {
	Match    []Match   `json:"match"`
	Handle   []Handler `json:"handle"`
	Terminal bool      `json:"terminal,omitempty"`
}

type Match struct {
	Host []string `json:"host"`
}

type Handler struct {
	Handler    string      `json:"handler"`
	Upstreams  []Upstream  `json:"upstreams,omitempty"`
	Routes     []Route     `json:"routes,omitempty"`
	Terminal   bool        `json:"terminal,omitempty"`
}

type Upstream struct {
	Dial string `json:"dial"`
}
