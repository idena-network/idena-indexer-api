SELECT tb.balance,
       coalesce(t.name, '')    "name",
       coalesce(t.symbol, '')  symbol,
       coalesce(t.decimals, 0) decimals
FROM token_balances tb
         LEFT JOIN tokens t ON t.contract_address_id = tb.contract_address_id
WHERE tb.address = lower($1)
  AND tb.contract_address_id = (SELECT id FROM addresses WHERE lower(address) = lower($2))