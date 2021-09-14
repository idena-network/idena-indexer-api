SELECT vr.delegator_address_id,
       a.address                                 delegator_address,
       vr.total_balance,
       coalesce(vr.validation_balance, 0)        validation_balance,
       coalesce(vr.flips_balance, 0)             flips_balance,
       coalesce(vr.invitations_balance, 0)       invitations_balance,
       coalesce(vr.invitations2_balance, 0)      invitations2_balance,
       coalesce(vr.invitations3_balance, 0)      invitations3_balance,
       coalesce(vr.saved_invites_balance, 0)     saved_invites_balance,
       coalesce(vr.saved_invites_win_balance, 0) saved_invites_win_balance,
       coalesce(vr.reports_balance, 0)           reports_balance
FROM delegatee_validation_rewards vr
         JOIN addresses a ON a.id = vr.delegator_address_id
WHERE vr.epoch = $1
    AND vr.delegatee_address_id = (SELECT id FROM addresses WHERE lower(address) = lower($2))
    AND $5::bigint IS NULL
   OR vr.total_balance = $4 AND vr.delegator_address_id >= $5
   OR vr.total_balance < $4
ORDER BY vr.total_balance DESC, vr.delegator_address_id
LIMIT $3
