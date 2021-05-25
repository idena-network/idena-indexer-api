SELECT voting_a.address  oracle_voting_address,
       olc.value,
       success_a.address success_address,
       fail_a.address    fail_address
FROM contracts c
         JOIN oracle_lock_contracts olc ON olc.contract_tx_id = c.tx_id
         JOIN addresses voting_a ON voting_a.id = olc.oracle_voting_address_id
         JOIN addresses success_a ON success_a.id = olc.success_address_id
         JOIN addresses fail_a ON fail_a.id = olc.fail_address_id
WHERE c.contract_address_id = (SELECT id FROM addresses WHERE lower(address) = lower($1))