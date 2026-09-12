package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A skill de publicação viaja no binário: é o que faz o botão "publish"
// funcionar numa máquina que nunca instalou skill nenhuma.
func TestBuiltinTrazAPostReport(t *testing.T) {
	list := Builtin()
	if len(list) == 0 {
		t.Fatal("o binário devia trazer pelo menos uma skill")
	}
	var achou bool
	for _, s := range list {
		if s.Name == "bazel-post-report" {
			achou = true
			if !s.Builtin {
				t.Error("skill embarcada devia estar marcada como tal")
			}
			if s.Description == "" {
				t.Error("a descrição sai do frontmatter e é o que a página mostra")
			}
		}
	}
	if !achou {
		t.Errorf("bazel-post-report devia estar embarcada: %+v", list)
	}

	if !IsBuiltin("bazel-post-report") || !IsBuiltin("/bazel-post-report") {
		t.Error("IsBuiltin devia reconhecer com e sem a barra")
	}
	if IsBuiltin("post-report") {
		t.Error("a post-report do usuário não é embarcada — são skills diferentes, de propósito")
	}
}

// Materialize escreve no clone, onde o Claude Code procura as skills do
// projeto. Um nível só abaixo de .claude/skills: aninhar não é descoberto.
func TestMaterializeEscreveNoClone(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git", "info"), 0o755); err != nil {
		t.Fatalf("preparando o clone falso: %v", err)
	}

	if err := Materialize(dir); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	skill := filepath.Join(dir, ".claude", "skills", "bazel-post-report", "SKILL.md")
	data, err := os.ReadFile(skill)
	if err != nil {
		t.Fatalf("a skill devia estar em %s: %v", skill, err)
	}
	if !strings.Contains(string(data), "name: bazel-post-report") {
		t.Error("o SKILL.md escrito devia ser o embarcado")
	}

	// O clone é descartável, mas um agente que roda `git status` nele não tem
	// por que ver a skill que o Bazel acabou de escrever.
	exclude, err := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude"))
	if err != nil {
		t.Fatalf("exclude: %v", err)
	}
	if !strings.Contains(string(exclude), ".claude/") {
		t.Errorf("o .claude devia entrar no exclude do clone: %q", exclude)
	}

	// Rodar de novo é idempotente: mesma skill, uma linha só no exclude.
	if err := Materialize(dir); err != nil {
		t.Fatalf("Materialize duas vezes: %v", err)
	}
	exclude2, _ := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude"))
	if strings.Count(string(exclude2), ".claude/") != 1 {
		t.Errorf("o exclude não devia acumular linha: %q", exclude2)
	}
}

// Sem diretório de trabalho não há onde escrever, e escrever na pasta de quem
// rodou o Bazel seria sujar um projeto alheio.
func TestMaterializeRecusaSemDiretorio(t *testing.T) {
	if err := Materialize("  "); err == nil {
		t.Error("Materialize sem diretório devia dar erro")
	}
}

// Pasta sem repositório não quebra: o exclude é o melhor esforço.
func TestMaterializeSemGit(t *testing.T) {
	dir := t.TempDir()
	if err := Materialize(dir); err != nil {
		t.Fatalf("Materialize sem .git: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "skills", "bazel-post-report", "SKILL.md")); err != nil {
		t.Errorf("a skill devia ter sido escrita mesmo assim: %v", err)
	}
}
