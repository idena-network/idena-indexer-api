SELECT tb.address,
       tb.balance,
       coalesce(t.name, '')    "name",
       coalesce(t.symbol, '')  symbol,
       coalesce(t.decimals, 0) decimals
FROM token_balances tb
         LEFT JOIN tokens t ON t.contract_address_id = tb.contract_address_id
WHERE tb.contract_address_id = (SELECT id FROM addresses WHERE lower(address) = lower($1))
  AND ($3::text IS NULL OR tb.balance = $4 AND lower(tb.address) >= lower($3) OR tb.balance < $4)
ORDER BY tb.balance DESC, lower(tb.address)
LIMIT $2;