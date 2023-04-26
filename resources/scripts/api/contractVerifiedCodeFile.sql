SELECT "data"
FROM contract_verifications
WHERE contract_address_id = (SELECT id FROM addresses WHERE lower(address) = lower($1))
  AND state = 1