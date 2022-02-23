select coalesce(sum(1), 0)                                                      all_count,
       coalesce(sum((case when invite_tx_id is not null then 1 else 0 end)), 0) used_count
from transactions t
         left join activation_txs ui on ui.invite_tx_id = t.id
where t.type = (select id from dic_tx_types where name = 'InviteTx')
  and t.id >= (select min_tx_id from epoch_summaries where epoch = $1)
  and t.id <= (select max_tx_id from epoch_summaries where epoch = $1)
