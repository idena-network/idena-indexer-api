SELECT coalesce((SELECT p.size
                 FROM pool_sizes p
                          JOIN addresses a ON a.id = p.address_id AND lower(a.address) = lower($1)), 0)
