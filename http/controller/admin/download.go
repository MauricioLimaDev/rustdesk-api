package admin

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
)

// Download serve os executáveis do cliente personalizado (Appelsoft).
// Os arquivos ficam na VPS, montados em RUSTDESK_API_DOWNLOADS_DIR (padrão /app/downloads):
//
//	cliente.exe -> versão das máquinas atendidas (também tem link público pelo Caddy)
//	equipe.exe  -> versão do time; só sai por aqui, com usuário logado
//	equipe-mac-arm64.dmg -> equipe para Mac Apple Silicon (experimental)
type Download struct {
}

type downloadItem struct {
	Tipo         string `json:"tipo"`
	Nome         string `json:"nome"`
	Disponivel   bool   `json:"disponivel"`
	Tamanho      int64  `json:"tamanho"`
	Atualizado   string `json:"atualizado"`
	LinkPublico  string `json:"link_publico"`
	Experimental bool   `json:"experimental"`
	Versao       string `json:"versao"`
}

// Versões Mac da equipe são experimentais (sem assinatura da Apple).
var downloadArquivos = map[string]struct {
	arquivo, nome, publico string
	experimental           bool
}{
	"cliente":          {"cliente.exe", "SuporteAppelsoft.exe", "/download/cliente", false},
	"equipe":           {"equipe.exe", "SuporteAppelsoft-Equipe.exe", "", false},
	"equipe-mac-arm64": {"equipe-mac-arm64.dmg", "SuporteAppelsoft-Equipe-mac-arm64.dmg", "", true},
}

var downloadOrdem = []string{"cliente", "equipe", "equipe-mac-arm64"}

func downloadsDir() string {
	if d := os.Getenv("RUSTDESK_API_DOWNLOADS_DIR"); d != "" {
		return d
	}
	return "/app/downloads"
}

// Info lista as versões disponíveis para download.
func (d *Download) Info(c *gin.Context) {
	itens := make([]downloadItem, 0, len(downloadOrdem))
	for _, tipo := range downloadOrdem {
		a := downloadArquivos[tipo]
		item := downloadItem{Tipo: tipo, Nome: a.nome, LinkPublico: a.publico, Experimental: a.experimental}
		if st, err := os.Stat(filepath.Join(downloadsDir(), a.arquivo)); err == nil && !st.IsDir() {
			item.Disponivel = true
			item.Tamanho = st.Size()
			item.Atualizado = st.ModTime().Format("2006-01-02 15:04")
			// Número da versão gravado pelo build ao publicar (<arquivo>.versao).
			if v, err := os.ReadFile(filepath.Join(downloadsDir(), a.arquivo+".versao")); err == nil {
				item.Versao = strings.TrimSpace(string(v))
			}
		}
		itens = append(itens, item)
	}
	response.Success(c, itens)
}

// Arquivo envia o executável. O tipo vem só da lista fixa acima (sem caminho livre).
func (d *Download) Arquivo(c *gin.Context) {
	a, ok := downloadArquivos[c.Param("tipo")]
	if !ok {
		response.Fail(c, 404, response.TranslateMsg(c, "ItemNotFound"))
		return
	}
	caminho := filepath.Join(downloadsDir(), a.arquivo)
	if st, err := os.Stat(caminho); err != nil || st.IsDir() {
		response.Fail(c, 404, response.TranslateMsg(c, "ItemNotFound"))
		return
	}
	c.FileAttachment(caminho, a.nome)
}
