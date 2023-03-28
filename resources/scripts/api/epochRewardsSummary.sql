SELECT tr.epoch,
       tr.total,
       tr.validation,
       tr.flips,
       coalesce(tr.flips_extra, 0)       extra_flips,
       tr.invitations,
       coalesce(tr.reports, 0)           reports,
       coalesce(tr.candidate, 0)         candidate,
       coalesce(tr.staking, 0)           staking,
       tr.foundation,
       tr.zero_wallet,
       tr.validation_share,
       tr.flips_share,
       coalesce(tr.flips_extra_share, 0) extra_flips_share,
       tr.invitations_share,
       coalesce(tr.reports_share, 0)     reports_share,
       coalesce(tr.candidate_share, 0)   candidate_share,
       coalesce(tr.staking_share, 0)     staking_share,
       es.block_count                    epoch_duration,
       coalesce(prev1.block_count, 0)    prev_epoch_duration_1,
       coalesce(prev2.block_count, 0)    prev_epoch_duration_2
FROM total_rewards tr
         LEFT JOIN epoch_summaries es
                   ON es.epoch = tr.epoch
         LEFT JOIN epoch_summaries prev1 ON prev1.epoch = tr.epoch - 1
         LEFT JOIN epoch_summaries prev2 ON prev2.epoch = tr.epoch - 2
WHERE tr.epoch = $1