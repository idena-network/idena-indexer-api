SELECT bu.id,
       bu.balance_old,
       bu.stake_old,
       coalesce(bu.penalty_old, 0)                       penalty_old,
       bu.balance_new,
       bu.stake_new,
       coalesce(bu.penalty_new, 0)                       penalty_new,
       dicr.name                                         reason,
       b.height                                          block_height,
       b.hash                                            block_hash,
       b.timestamp                                       block_timestamp,
       coalesce(t.hash, '')                              tx_hash,
       coalesce(lb.height, 0)                            last_block_height,
       coalesce(lb.hash, '')                             last_block_hash,
       coalesce(lb.timestamp, 0)                         last_block_timestamp,
       coalesce(bu.committee_reward_share, 0)            committee_reward_share,
       coalesce(bu.blocks_count, 0)                      blocks_count,
       (case when bu.reason = 4 then b.epoch else 0 end) epoch,
       coalesce(contract_a.address, '')                  contract_address
FROM balance_updates bu
         LEFT JOIN transactions t ON t.id = bu.tx_id
         LEFT JOIN dic_balance_update_reasons dicr ON dicr.id = bu.reason
         LEFT JOIN blocks b ON b.height = bu.block_height
         LEFT JOIN blocks lb ON lb.height = bu.last_block_height
         LEFT JOIN contracts c ON bu.reason = 10 AND t.type = 15 AND c.tx_id = t.id
         LEFT JOIN addresses contract_a ON bu.reason = 10 AND (t.type in (16, 17) AND contract_a.id = t.to OR
                                                               t.type = 15 AND contract_a.id = c.contract_address_id)
WHERE ($3::bigint IS NULL OR bu.id <= $3)
  AND bu.address_id = (SELECT id FROM addresses WHERE lower(address) = lower($1))
ORDER BY bu.id DESC
LIMIT $2