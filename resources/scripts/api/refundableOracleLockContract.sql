SELECT voting_a.address                 oracle_voting_address,
       rolc.value,
       coalesce(success_a.address, '')  success_address,
       coalesce(fail_a.address, '')     fail_address,
       rolc.deposit_deadline            deposit_deadline,
       rolc.oracle_voting_fee           oracle_voting_fee,
       rolc.refund_delay                refund_delay,
       coalesce(pushes.refund_block, 0) refund_block,
       head_block.height                head_block_height,
       head_block.timestamp             head_block_timestamp,
       terminationb.timestamp           terminationTxTimestamp
FROM contracts c
         JOIN refundable_oracle_lock_contracts rolc ON rolc.contract_tx_id = c.tx_id
         JOIN addresses voting_a ON voting_a.id = rolc.oracle_voting_address_id
         LEFT JOIN refundable_oracle_lock_contract_call_pushes pushes
                   ON pushes.ol_contract_tx_id = c.tx_id AND coalesce(pushes.refund_block, 0) > 0
         LEFT JOIN addresses success_a ON success_a.id = rolc.success_address_id
         LEFT JOIN addresses fail_a ON fail_a.id = rolc.fail_address_id
         LEFT JOIN refundable_oracle_lock_contract_terminations terminations
                   ON terminations.ol_contract_tx_id = c.tx_id
         LEFT JOIN transactions terminationt ON terminationt.id = terminations.termination_tx_id
         LEFT JOIN blocks terminationb on terminationb.height = terminationt.block_height,
     (SELECT height, timestamp FROM blocks ORDER BY height DESC LIMIT 1) head_block
WHERE c.contract_address_id = (SELECT id FROM addresses WHERE lower(address) = lower($1))