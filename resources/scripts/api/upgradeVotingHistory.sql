SELECT block_height, "timestamp", votes
FROM upgrade_voting_history
WHERE NOT exists(SELECT 1 FROM upgrade_voting_short_history_summary WHERE upgrade = $1)
  AND upgrade = $1
UNION
SELECT sh.block_height, h."timestamp", sh.votes
FROM upgrade_voting_short_history sh
         JOIN upgrade_voting_history h on h.block_height = sh.block_height and h.upgrade = sh.upgrade
WHERE sh.upgrade = $1
UNION
(SELECT block_height, "timestamp", votes
 FROM upgrade_voting_history
 WHERE upgrade = $1
 ORDER BY block_height DESC
 LIMIT 1)
ORDER BY block_height