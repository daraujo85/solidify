// Rubrica real do peer review (SAI-129B), embarcada no binário.
//
// Cópia de prompts/peer-review.md (adaptada na seção Output pro
// schema canônico SAI-129A). Cópia deliberada, não reuso de
// internal/mcpserver.LoadPrompt: aquele loader faz os.ReadFile de
// caminho relativo ("prompts/peer-review.md"), que quebra quando o
// cwd do solidify é o repo ALVO sendo analisado, não o diretório de
// instalação do próprio solidify. go:embed elimina essa dependência
// de cwd. Tradeoff aceito e sinalizado: duas cópias do texto da
// rubrica existem agora (prompts/peer-review.md, dead code via
// internal/mcpserver, e este embed) — internal/mcpserver/prompt.go
// não foi tocado, fica fora de escopo do SAI-129B.
package peer

import _ "embed"

//go:embed peer_review_rubric.md
var PeerReviewRubric string
