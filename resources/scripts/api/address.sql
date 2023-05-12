select a.Address,
       coalesce(b.Balance, '0')                                               balance,
       coalesce(b.Stake, '0')                                                 stake,
       (select count(*) from transactions where "from" = a.id or "to" = a.id) tx_count,
       coalesce(asum.flips, 0)                                                flips,
       coalesce(asum.wrong_words_flips, 0)                                    wrong_words_flips,
       (SELECT count(*) FROM token_balances WHERE lower(address) = lower($1)) token_count
from addresses a
         left join balances b on b.address_id = a.id
         left join address_summaries asum on asum.address_id = a.id
where lower(a.Address) = lower($1)