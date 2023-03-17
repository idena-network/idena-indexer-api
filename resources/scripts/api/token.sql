SELECT a.address, coalesce(t.name, '') "name", coalesce(t.Symbol, '') symbol, coalesce(t.Decimals, 0) decimals
FROM tokens t
         JOIN addresses a ON a.id = t.contract_address_id AND lower(a.address) = lower($1)
