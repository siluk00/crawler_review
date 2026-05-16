# crawler_review — Revisão de Código

Este repositório contém a análise, correção e justificativa técnica de um crawler escrito em Go que coleta livros do site [books.toscrape.com](http://books.toscrape.com).

O código original funciona em condições ideais, mas apresenta problemas sérios que o tornariam instável em produção. Este documento descreve cada problema encontrado, seu impacto real, e a correção aplicada.

---

## Problemas encontrados

### #1 — Paginação começa na página 2

**Severidade: crítica**

```go
// original
page := "page-2.htm"
```

A primeira página do catálogo nunca é coletada. O problema é silencioso — o programa não emite erro, não avisa, simplesmente ignora os primeiros 20 livros. Em produção, isso significa que parte do catálogo não existe no resultado e ninguém percebe.

**Correção:** iniciar pela primeira página.

```go
page := "page-1.htm"
```

---


### #2 — Estado global mutável

**Severidade: média**

```go
// original
var results []Book  // variável global
```

`results` é uma variável global que em caso de escalabilidade para múltiplas gorotina geraria dois problemas:

1. **Chamadas múltiplas acumulam dados:** chamar `crawl()` duas vezes dobra os resultados sem resetar o estado anterior.
2. **Race condition em concorrência:** se dois requests chegarem simultaneamente em um servidor HTTP, ambos escrevem em `results` ao mesmo tempo. Em Go, isso é undefined behavior — pode causar panic, dados corrompidos, ou resultados misturados entre clientes diferentes.

**Correção:** transformar `results` em variável local dentro de `crawl()` e retorná-la explicitamente.

```go
func crawl() ([]Book, error) {
    var results []Book
    // ...
    return results, nil
}
```

---

### #3 — Sem timeout no HTTP client

**Severidade: crítica**

```go
// original
resp, err := http.DefaultClient.Do(req)
```

`http.DefaultClient` não tem timeout configurado. Se o servidor demorar para responder ou a conexão travar, a goroutine fica bloqueada indefinidamente. Em produção, uma única página lenta pode suspender o processo inteiro sem possibilidade de recuperação.

**Correção:** criar um client dedicado com timeout explícito.

```go
httpClient := &http.Client{Timeout: 15 * time.Second}
```

---

### #4 — Link da página errado

**Severidade: crítica**

```go
// original
page := "page-2.htm"
```

A página procurada é chamada com a extensão `.htm` mas no site original o correto é `.html`. Isso faz com que o crawler nunca comece.

**Correção:** corrigir a extensão.

```go
page := "page-1.html"
```

---

## Resumo dos problemas

| # | Problema | Severidade | Impacto em produção |
|---|----------|------------|---------------------|
| 1 | Paginação começa na página 2 | Crítica | Perda silenciosa de dados |
| 2 | Estado global mutável | Média | Possível race condition e acumulo entre chamadas em escalabilidade |
| 3 | Sem timeout no HTTP client | Crítica | Processo suspenso indefinidamente |
| 4 | Extensão errada da página | Crítica | O crawler nunca começa |

---

## Dependencias

| Pacote | Motivo |
|--------|--------|
| `golang.org/x/net/html` | Parser HTML oficial do projeto Go |
