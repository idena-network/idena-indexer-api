SELECT mc.min_votes, mc.max_votes
FROM contracts c
         JOIN multisig_contracts mc ON mc.contract_tx_id = c.tx_id
WHERE c.contract_address_id = (SELECT id FROM addresses WHERE lower(address) = lower($1))