SELECT e.epoch,
       e.validation_time,
       es.validated_count,
       es.block_count,
       es.empty_block_count,
       es.tx_count,
       es.invite_count,
       es.flip_count,
       es.burnt,
       es.minted,
       es.total_balance,
       es.total_stake,
       coalesce(trew.total, 0)                  total_reward,
       coalesce(trew.validation, 0)             validation_reward,
       coalesce(trew.flips, 0)                  flips_reward,
       coalesce(trew.invitations, 0)            invitations_reward,
       coalesce(trew.reports, 0)                reports_reward,
       coalesce(trew.candidate, 0)              candidate_reward,
       coalesce(trew.staking, 0)                staking_reward,
       coalesce(trew.foundation, 0)             foundation_payout,
       coalesce(trew.zero_wallet, 0)            zero_wallet_payout,
       coalesce(preves.min_score_for_invite, 0) min_score_for_invite,
       coalesce(es.candidate_count, 0)          candidate_count,
       e.discrimination_stake_threshold
FROM epochs e
         LEFT JOIN epoch_summaries es ON es.epoch = e.epoch
         LEFT JOIN epoch_summaries preves ON preves.epoch = e.epoch - 1
         LEFT JOIN total_rewards trew ON trew.epoch = e.epoch
WHERE $2::bigint IS NULL
   OR e.epoch <= $2::bigint
ORDER BY e.epoch DESC
LIMIT $1
