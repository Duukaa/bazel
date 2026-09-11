package server

import (
	"bytes"
	"fmt"
	"html"
	"strings"

	"github.com/beroni/bazel/internal/store"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// md converte markdown em HTML. Sem goldmark.WithUnsafe(): HTML cru no
// markdown sai escapado, não executado.
var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		extension.Footnote,
	),
)

// policy limpa o HTML depois de convertido.
//
// O markdown não vem só do agente: a descrição do PR é escrita por quem abriu
// o PR. goldmark escapa HTML cru, mas ainda emite o link que o markdown pediu
// — e `[clica](javascript:...)` num servidor que dispara review e comenta no
// GitHub no seu nome não é um link qualquer. UGCPolicy corta esquema que não
// seja http/https/mailto.
var policy = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	// Checkbox de task list do GFM, que o UGCPolicy tiraria.
	p.AllowAttrs("type", "checked", "disabled").Matching(bluemonday.Paragraph).OnElements("input")
	// Sem isto o class="language-go" some e o bloco de código perde a marca.
	p.AllowAttrs("class").OnElements("code", "pre", "span", "div", "li")
	return p
}()

// renderMarkdown devolve o HTML seguro para jogar no innerHTML da página.
func renderMarkdown(src string) string {
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return "<pre>" + html.EscapeString(src) + "</pre>"
	}
	return string(policy.SanitizeBytes(buf.Bytes()))
}

// renderReview converte um review em HTML com cada achado embrulhado numa
// <section class="finding" data-finding="N">. A página põe a caixa de
// marcar em cada uma, e o N é o mesmo índice que o publish recebe em `skip`
// — os dois lados cortam o markdown com o mesmo store.SplitFindings, então
// a caixa desmarcada na tela é exatamente o bloco que sai do que vai ao PR.
//
// O embrulho fica fora da sanitização: é HTML nosso, não do agente.
func renderReview(body string) string {
	parts := store.SplitFindings(body)
	if len(parts) <= 1 {
		return renderMarkdown(body)
	}
	var b strings.Builder
	idx := 0
	for _, p := range parts {
		if !p.Finding {
			b.WriteString(renderMarkdown(p.Text))
			continue
		}
		fmt.Fprintf(&b, `<section class="finding" data-finding="%d">`, idx)
		b.WriteString(renderMarkdown(p.Text))
		b.WriteString("</section>")
		idx++
	}
	return b.String()
}
