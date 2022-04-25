SELECT vr.epoch,
       coalesce(vr.validation_balance, 0)        validation_balance,
       coalesce(vr.flips_balance, 0)             flips_balance,
       coalesce(vr.invitations_balance, 0)       invitations_balance,
       coalesce(vr.invitations2_balance, 0)      invitations2_balance,
       coalesce(vr.invitations3_balance, 0)      invitations3_balance,
       coalesce(vr.saved_invites_balance, 0)     saved_invites_balance,
       coalesce(vr.saved_invites_win_balance, 0) saved_invites_win_balance,
       coalesce(vr.reports_balance, 0)           reports_balance,
       coalesce(vr.candidate_balance, 0)         candidate_balance,
       coalesce(vr.staking_balance, 0)           staking_balance,
       vr.delegators
FROM delegatee_total_validation_rewards vr
WHERE vr.epoch = $1
  AND vr.delegatee_address_id = (SELECT id FROM addresses WHERE lower(address) = lower($2))