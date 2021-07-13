SELECT 1, coalesce(reported_flips, 0)
FROM epoch_summaries
WHERE epoch = $1;