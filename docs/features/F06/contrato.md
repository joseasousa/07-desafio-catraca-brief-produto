# F06 — Contrato

Subir com `./scripts/up` e usar os dois organizadores, um participante e a chave do README. Preparar ingressos web e API em eventos publicados com vagas.

- F06-1 → X26. Em outro evento e para código inexistente, conferir Ingresso inválido para este evento. Para ingresso não confirmado, conferir Ingresso não confirmado. Conferir que usuários sem acesso não podem executar ações por URL direta.
- F06-2 → X36. Usar os cartões com final 0003 e 0001. Depois da confirmação rápida e antes de 30 s, cancelar. Verificar desaparecimento da vitrine e da API, compra 404 event_not_available, edição e publicação bloqueadas, status refunded e cancelled em até 10 s e os mesmos status após 45 s. Conferir Evento cancelado e painel com zero confirmados, pendentes, check-ins, vagas e receita. Estornados corresponde aos ingressos antes confirmados.
- F06-3 → X46. Confirmar compra web e verificar Entrada liberada no primeiro check-in e Ingresso já utilizado no segundo. Contar uma entrada e verificar que o cancelamento estorna inclusive ingresso utilizado.
- F06-4 → X56. Repetir check-in e estorno com compra API. Em evento de três vagas, comprar dois ingressos de R$ 100,00 pelos dois canais e um de R$ 150,00 pela API após editar o preço. Conferir três confirmados, zero vagas e R$ 350,00 de receita antes de cancelar.
