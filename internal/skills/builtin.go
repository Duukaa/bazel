package skills

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// builtinFS são as skills que viajam dentro do binário do Bazel.
//
// A publicação precisava de uma skill instalada em ~/.claude/skills para
// funcionar, e isso fazia o botão "publish" mentir em toda máquina que não
// fosse a de quem escreveu a skill: o agente rodava, com autorização declarada
// para escrever no PR, e sem instrução nenhuma. Embarcar é o que faz o padrão
// de fábrica ser verdade em qualquer lugar.
//
//go:embed builtin
var builtinFS embed.FS

// builtinRoot é a pasta dentro do embed.
const builtinRoot = "builtin"

// SkillsSubdir é onde o Claude Code procura as skills de um projeto, relativo
// ao diretório de trabalho do agente. Um nível só: `.claude/skills/<nome>/`
// é descoberto, `.claude/skills/<grupo>/<nome>/` não.
const SkillsSubdir = ".claude/skills"

// Builtin lista as skills embarcadas, em ordem alfabética.
func Builtin() []Skill {
	entries, err := builtinFS.ReadDir(builtinRoot)
	if err != nil {
		return nil
	}
	out := make([]Skill, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		s := Skill{Name: e.Name(), Dir: builtinRoot + "/" + e.Name(), Builtin: true}
		if data, err := builtinFS.ReadFile(s.Dir + "/SKILL.md"); err == nil {
			s.Name, s.Description = frontmatterOf(string(data), e.Name())
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// IsBuiltin diz se um nome é de skill embarcada. É o que permite a página
// mostrar essas como instaladas sem procurá-las no disco: elas estão no
// binário, e não há o que instalar.
func IsBuiltin(name string) bool {
	name = strings.TrimPrefix(strings.TrimSpace(name), "/")
	for _, s := range Builtin() {
		if strings.EqualFold(s.Name, name) {
			return true
		}
	}
	return false
}

// Materialize escreve as skills embarcadas em <dir>/.claude/skills, que é onde
// o Claude Code as encontra quando roda com dir como diretório de trabalho.
//
// dir é o clone temporário do PR: a skill nasce e morre com ele, e a máquina
// de quem roda o Bazel não ganha nada em ~/.claude/skills que a pessoa não
// tenha pedido. Os arquivos também entram no exclude do clone, para não
// aparecerem no `git status` de um agente que o consulte.
func Materialize(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("no working directory to materialize the built-in skills into")
	}
	err := fs.WalkDir(builtinFS, builtinRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(builtinRoot, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		dest := filepath.Join(dir, SkillsSubdir, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		data, err := builtinFS.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0o644)
	})
	if err != nil {
		return fmt.Errorf("writing the built-in skills into %s: %w", dir, err)
	}
	excludeClaude(dir)
	return nil
}

// excludeClaude põe .claude/ no exclude do clone. É melhor esforço: o clone é
// descartável, e um agente que roda `git status` não tem por que ver a skill
// que o Bazel acabou de escrever ali. Sem repositório, não há o que fazer.
func excludeClaude(dir string) {
	info := filepath.Join(dir, ".git", "info")
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return
	}
	if err := os.MkdirAll(info, 0o755); err != nil {
		return
	}
	path := filepath.Join(info, "exclude")
	atual, _ := os.ReadFile(path)
	if strings.Contains(string(atual), "\n.claude/") || strings.HasPrefix(string(atual), ".claude/") {
		return
	}
	linha := ".claude/\n"
	if len(atual) > 0 && !strings.HasSuffix(string(atual), "\n") {
		linha = "\n" + linha
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(linha)
}
