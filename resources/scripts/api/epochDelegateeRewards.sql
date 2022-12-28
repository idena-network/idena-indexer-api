SELECT vr.delegator_address_id,
       coalesce(a.address, '')                   delegator_address,
       coalesce(prevdics.name, '')               prev_state,
       coalesce(dics.name, '')                   state,
       vr.total_balance,
       coalesce(vr.validation_balance, 0)        validation_balance,
       coalesce(vr.flips_balance, 0)             flips_balance,
       coalesce(vr.extra_flips_balance, 0)       extra_flips_balance,
       coalesce(vr.invitations_balance, 0)       invitations_balance,
       coalesce(vr.invitations2_balance, 0)      invitations2_balance,
       coalesce(vr.invitations3_balance, 0)      invitations3_balance,
       coalesce(vr.invitee1_balance, 0)          invitee1_balance,
       coalesce(vr.invitee2_balance, 0)          invitee2_balance,
       coalesce(vr.invitee3_balance, 0)          invitee3_balance,
       coalesce(vr.saved_invites_balance, 0)     saved_invites_balance,
       coalesce(vr.saved_invites_win_balance, 0) saved_invites_win_balance,
       coalesce(vr.reports_balance, 0)           reports_balance,
       coalesce(vr.candidate_balance, 0)         candidate_balance,
       coalesce(vr.staking_balance, 0)           staking_balance
FROM delegatee_validation_rewards vr
         LEFT JOIN addresses a ON a.id = vr.delegator_address_id
         LEFT JOIN epoch_identities ei ON ei.address_id = vr.delegator_address_id AND ei.epoch = vr.epoch
         LEFT JOIN address_states s ON s.id = ei.address_state_id
         LEFT JOIN dic_identity_states dics ON dics.id = s.state
         LEFT JOIN address_states prevs ON prevs.id = s.prev_id
         LEFT JOIN dic_identity_states prevdics ON prevdics.id = prevs.state
WHERE vr.epoch = $1
  AND vr.delegatee_address_id = (SELECT id FROM addresses WHERE lower(address) = lower($2))
  AND ($5::bigint IS NULL
    OR vr.total_balance = $4 AND vr.delegator_address_id >= $5
    OR vr.total_balance < $4)
ORDER BY vr.total_balance DESC, vr.delegator_address_id
LIMIT $3
