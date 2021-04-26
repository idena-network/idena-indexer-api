SELECT block_height, votes
FROM upgrade_voting_history
WHERE NOT exists(SELECT 1 FROM upgrade_voting_short_history_summary WHERE upgrade = $1)
  AND upgrade = $1
UNION
SELECT block_height, votes
FROM upgrade_voting_short_history
WHERE upgrade = $1
UNION
(SELECT block_height, votes
 FROM upgrade_voting_history
 WHERE upgrade = $1
 ORDER BY block_height DESC
 LIMIT 1)
ORDER BY block_height