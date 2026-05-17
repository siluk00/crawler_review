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

`results` é uma variável global que em caso de escalabilidade para múltiplas goroutines geraria dois problemas:

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

### #5 — Logging com `fmt.Println` e `log.Println` em vez de logging estruturado

**Severidade: baixa**

```go
// original
fmt.Println("Coletando:", page)
// ...
log.Println("Erro:", err)
```

Mensagens de log em texto livre são difíceis de filtrar e correlacionar em produção. Ferramentas de observabilidade como Datadog, Loki e CloudWatch esperam logs estruturados em key-value — uma string plana como `"Coletando: page-3.htm"` não é indexável nem consultável.

`log.Println` e `fmt.Println` também misturam dois propósitos diferentes no mesmo canal de saída, sem distinção de nível (info, error, debug).

**Correção:** substituir por `slog`, o logger estruturado oficial do Go desde 1.21.

```go
// progresso de paginação
slog.Info("Coletando:", "page", page)

// erro de rede
slog.Error("erro ao buscar página",
    "page", page,
    "coletados", len(results),
    "erro", err,
)

// conclusão
slog.Info("crawl completado", "total", len(results))
```

---

### #6 — Última página é parseada duas vezes

**Severidade: crítica**

```go
// original
parseBooks(doc)  // primeira vez, no início do loop

next := getNextPage(doc)
if next == "" {
    parseBooks(doc)  // segunda vez, quando não há próxima página
}
```

Quando o crawler chega na última página, `parseBooks` é chamado duas vezes no mesmo `doc`. Todos os livros da última página entram duplicados em `results`. O dado final está corrompido sem qualquer indicação de erro.

**Correção:** remover o bloco `if next == ""` inteiramente. O parse já aconteceu no início do loop.

---

### #7 — Erro de rede descarta dados já coletados

**Severidade: crítica**

```go
// original
if err != nil {
    log.Println("Erro:", err)
    break
}
```

Se a rede falhar na página 15 de 50, o `break` encerra o loop e as 14 páginas já coletadas são descartadas silenciosamente. Em produção, o resultado seria um retorno vazio ou incompleto sem qualquer indicação de progresso.

O comportamento correto depende do contexto, mas em geral há duas opções razoáveis: retornar o parcial coletado junto com o erro (resultado melhor do que nada), ou implementar retry com backoff. Para este escopo, retornar o parcial com o erro é a correção mínima adequada.

```go
if err != nil {
    slog.Error("erro ao buscar página, retornando resultados parciais",
        "page", page,
        "coletados", len(results),
        "erro", err,
    )
    return results, fmt.Errorf("pagina %s: %w", page, err)
}
```

---

### #8 — `getNextPage` nunca encontra o link da próxima página

**Severidade: crítica**

```go
// original
if n.FirstChild != nil && n.FirstChild.NextSibling != nil {
    for _, a := range n.FirstChild.NextSibling.Attr {
```

O HTML real do elemento de paginação é:

```html
<li class="next"><a href="page-2.html">next</a></li>
```

`n.FirstChild` já é o `<a>` diretamente. O código pula para `n.FirstChild.NextSibling`, que é `nil` — o `<a>` não tem irmão dentro do `<li>`. A condição nunca é satisfeita, `next` sempre retorna `""`, e o crawler para na primeira página sem erro e sem aviso.

**Correção:** acessar `n.FirstChild` diretamente.

```go
// depois
if n.FirstChild != nil {
    for _, a := range n.FirstChild.Attr {
```

---

## Resumo dos problemas

| # | Problema | Severidade | Impacto em produção |
|---|----------|------------|---------------------|
| 1 | Paginação começa na página 2 | Crítica | Perda silenciosa de dados |
| 2 | Estado global mutável | Média | Possível race condition e acúmulo entre chamadas em escalabilidade |
| 3 | Sem timeout no HTTP client | Crítica | Processo suspenso indefinidamente |
| 4 | Extensão errada da página | Crítica | O crawler nunca começa |
| 5 | Logging não estruturado | Baixa | Logs não indexáveis em produção |
| 6 | Última página parseada duas vezes | Crítica | Duplicação de dados no resultado |
| 7 | Erro de rede descarta parcial | Crítica | Perda total de dados coletados |
| 8 | `getNextPage` não lê o href | Crítica | Crawler para na primeira página |

---

## Dependências

| Pacote | Motivo |
|--------|--------|
| `golang.org/x/net/html` | Parser HTML oficial do projeto Go |
