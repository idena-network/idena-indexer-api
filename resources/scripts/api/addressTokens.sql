SELECT tb.contract_address_id,
       a.address               contract_address,
       tb.balance,
       coalesce(t.name, '')    "name",
       coalesce(t.symbol, '')  symbol,
       coalesce(t.decimals, 0) decimals
FROM token_balances tb
         LEFT JOIN addresses a ON a.id = tb.contract_address_id
         LEFT JOIN tokens t ON t.contract_address_id = tb.contract_address_id
WHERE tb.address = lower($1)
  AND ($3::bigint IS NULL OR tb.contract_address_id >= $3)
ORDER BY tb.contract_address_id
LIMIT $2
