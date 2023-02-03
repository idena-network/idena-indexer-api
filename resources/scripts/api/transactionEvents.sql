SELECT te.idx,
       te.event_name,
       te.data
FROM tx_events te
WHERE te.tx_id = (SELECT id FROM transactions WHERE lower(hash) = lower($1))
  AND ($3::integer IS NULL OR te.idx >= $3)
ORDER BY idx
LIMIT $2