select coalesce((select tx_count from epoch_summaries where epoch = $1), 0)