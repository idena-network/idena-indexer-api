SELECT vr.delegatee_address_id,
       coalesce(a.address, '')                   address,
       vr.total_balance,
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
         LEFT JOIN addresses a ON a.id = vr.delegatee_address_id
WHERE vr.epoch = $1
    AND $4::bigint IS NULL
   OR vr.total_balance = $3 AND vr.delegatee_address_id >= $4
   OR vr.total_balance < $3
ORDER BY vr.total_balance DESC, vr.delegatee_address_id
LIMIT $2