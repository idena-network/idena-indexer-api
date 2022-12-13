SELECT voting_a.address                oracle_voting_address,
       rolc.value,
       coalesce(success_a.address, '') success_address,
       coalesce(fail_a.address, '')    fail_address,
       rolc.deposit_deadline           deposit_deadline,
       rolc.oracle_voting_fee          oracle_voting_fee,
       rolc.refund_delay               refund_delay
FROM contracts c
         JOIN refundable_oracle_lock_contracts rolc ON rolc.contract_tx_id = c.tx_id
         JOIN addresses voting_a ON voting_a.id = rolc.oracle_voting_address_id
         LEFT JOIN addresses success_a ON success_a.id = rolc.success_address_id
         LEFT JOIN addresses fail_a ON fail_a.id = rolc.fail_address_id
WHERE c.contract_address_id = (SELECT id FROM addresses WHERE lower(address) = lower($1))