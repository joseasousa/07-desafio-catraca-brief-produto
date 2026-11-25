# F06 — Spec

Check-in e cancelamento escrevem atomicamente. Painel agrega ingressos pelo status atual e preço registrado. O PRD define os critérios X26 X36 X46 X56.

Reusar a URL de gestão da F02. Check-in verifica código e evento, cancelamento, confirmação e uso anterior nessa ordem. Checagem e gravação usam uma única transação. Evento cancelado pode mostrar Evento cancelado no lugar do formulário.

Cancelar converte pendentes em cancelados e confirmados em estornados imediatamente na mesma transação que marca o evento. O painel agrega em uma consulta consistente. Check-ins contam somente confirmados marcados, receita soma somente seus preços históricos e vagas são zero se o evento estiver cancelado.
