SELECT d.delegator_address_id,
       coalesce(d.birth_epoch, 9999),
       coalesce(delegatora.address, '')                                                  delegator_address,
       coalesce(dics.name, 'Undefined')                                                  state,
       (case when d.birth_epoch is null then 0 else cur_rpoch.epoch - d.birth_epoch end) age,
       coalesce(bal.stake, 0)                                                            stake
FROM delegations d
         LEFT JOIN addresses delegatora ON delegatora.id = d.delegator_address_id
         LEFT JOIN address_states s ON s.address_id = d.delegator_address_id AND s.is_actual
         LEFT JOIN dic_identity_states dics ON dics.id = s.state
         LEFT JOIN balances bal ON bal.address_id = d.delegator_address_id
        ,
     (SELECT max(epoch) epoch FROM epochs) cur_rpoch
WHERE d.delegatee_address_id = (SELECT id FROM addresses WHERE lower(address) = lower($4))
  AND ($2::bigint IS NULL OR coalesce(d.birth_epoch, 9999) = $3 AND d.delegator_address_id >= $2 OR
       coalesce(d.birth_epoch, 9999) > $3)
ORDER BY coalesce(d.birth_epoch, 9999), d.delegator_address_id
LIMIT $1