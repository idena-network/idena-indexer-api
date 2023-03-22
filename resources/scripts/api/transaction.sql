select b.epoch,
       b.height,
       b.hash,
       t.Hash,
       dtt.name                                                                                     "type",
       b.Timestamp,
       afrom.Address                                                                                "from",
       COALESCE(ato.Address, '')                                                                    "to",
       t.Amount,
       t.Tips,
       t.max_fee,
       t.Fee,
       t.size,
       coalesce(t.nonce, 0)                                                                         nonce,
       coalesce(atxs.balance_transfer,
                coalesce(ktxs.stake_transfer,
                         kitxs.stake_transfer))                                                     transfer,
       (case when online.tx_id is not null then true when offline.tx_id is not null then false end) become_online,
       tr.success                                                                                   tx_receipt_success,
       tr.gas_used                                                                                  tx_receipt_gas_used,
       tr.gas_cost                                                                                  tx_receipt_gas_cost,
       tr.method                                                                                    tx_receipt_method,
       tr.error_msg                                                                                 tx_receipt_error_msg,
       (case
            when tr.tx_id is not null
                then coalesce(adeploy.address, ato.address) end)                                    tx_receipt_contract_address,
       coalesce(tr.action_result, ''::bytea)                                                        action_result
from transactions t
         LEFT JOIN blocks b ON b.height = t.block_height
         LEFT JOIN addresses afrom ON afrom.id = t.from
         LEFT JOIN addresses ato ON ato.id = t.to
         LEFT JOIN dic_tx_types dtt ON dtt.id = t.Type
         LEFT JOIN activation_tx_transfers atxs ON atxs.tx_id = t.id AND t.type = 1
         LEFT JOIN kill_tx_transfers ktxs ON ktxs.tx_id = t.id AND t.type = 3
         LEFT JOIN kill_invitee_tx_transfers kitxs ON kitxs.tx_id = t.id AND t.type = 10
         LEFT JOIN become_online_txs online ON online.tx_id = t.id AND t.type = 9
         LEFT JOIN become_offline_txs offline ON offline.tx_id = t.id AND t.type = 9
         LEFT JOIN tx_receipts tr ON t.type in (15, 16, 17) AND tr.tx_id = t.id
         LEFT JOIN addresses adeploy ON t.type = 15 AND adeploy.id = tr.contract_address_id
WHERE lower(t.Hash) = lower($1)