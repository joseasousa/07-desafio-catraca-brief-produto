# Catraca

Vitrine pública, compra de um ingresso por participante ou parceiro e operação do evento pelo organizador dono.

| Wave | Feature | Provê | Consome |
|---|---|---|---|
| 1 | F01 Contas e ambiente | Sessão, papéis, seed, banco e harness | — |
| 2 | F02 Eventos | Gestão, vitrine e disponibilidade | F01 |
| 3 | F03 Compras e gateway | Compra, consultas, preço histórico e pagamentos | F02 |
| 4 | F04 Checkout web | Compra web e Meus ingressos | F01, F03 |
| 4 | F05 Parceiros | API autenticada | F02, F03 |
| 5 | F06 Operação | Check-in, cancelamento e painel | F02, F03, F04, F05 |

Grafo: F01 → F02, F04. F02 → F03, F05, F06. F03 → F04, F05, F06. F04 → F06. F05 → F06.

## Critérios

- A01 (F01) Clone limpo sobe com seed, login por papel, cadastro de participante e logout. Derrubar libera a porta e subir novamente preserva as credenciais.
- A02 (F02, R01, R02, R03, R04) Rejeitar dados inválidos e datas passadas ao criar ou editar. Rascunho fica fora da vitrine. Publicado aparece. Cada evento tem URL própria.
- X12 (F01 → F02, R13) Lista e URL direta acessíveis apenas ao dono, sem vazamento para outro organizador, participante ou visitante.
- X23 (F02 → F03, R02, R03, R05, R06, R07, R08) Comprar apenas evento publicado, capturar preço vigente, reservar antes da resposta, recusar quando lotado e liberar a vaga após recusa. Edição abaixo da ocupação falha. Duas rajadas de 30, em eventos novos com cinco vagas, retornam cinco aceites e 25 recusas cada.
- X14 (F01 → F04, R06, R09) Só participante autenticado compra pela web e vê os próprios ingressos.
- X34 (F03 → F04, R05, R06, R07, R08, R09) Compra web nasce pendente, exibe evento, status e código e mostra Ingressos esgotados ao atingir lotação.
- X25 (F02 → F05, R02, R05) Listagem reflete publicação, preço e vagas.
- X35 (F03 → F05, R06, R07, R08, R09) Compra e consulta API respeitam autenticação, precedência dos erros e formato fixo. Consulta admite ingressos dos dois canais.
- X26 (F02 → F06, R04, R10, R13) Operação pela URL do dono e mensagens exatas na ordem definida no brief.
- X36 (F03 → F06, R11, R12) Cancelamento impede compra, edição e republicação, retira de ambas as listas e converte confirmados e pendentes. Resposta tardia preserva status. Painel cancelado tem zero vagas, receita e check-ins.
- X46 (F04 → F06, R10, R11, R12) Ingresso web confirma, libera entrada uma vez, compõe painel e é estornado no cancelamento.
- X56 (F05 → F06, R10, R11, R12) Ingresso API segue as mesmas regras. Painel dos dois canais soma os valores históricos e exclui estornos.

## Fora de escopo

Deploy, gateway real, e-mail, notificações, app mobile, qualidade visual, cadastro de organizador, compra múltipla, assentos, cancelamento ou transferência pelo participante e atualização em tempo real.
