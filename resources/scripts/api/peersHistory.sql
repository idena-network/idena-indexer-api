SELECT ph.timestamp, ph.count
FROM peers_history ph
ORDER BY ph.timestamp DESC
LIMIT $1