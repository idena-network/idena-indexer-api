SELECT coalesce(ei.short_point, 0)                                    short_point,
       coalesce(ei.short_flips, 0)                                    short_flips,
       coalesce(least(ei.total_short_point, ei.total_short_flips), 0) total_short_point,
       coalesce(ei.total_short_flips, 0)                              total_short_flips,
       coalesce(ei.long_point, 0)                                     long_point,
       coalesce(ei.long_flips, 0)                                     long_flips,
       coalesce(ei.short_answers, 0)                                  short_answers,
       coalesce(ei.long_answers, 0)                                   long_answers,
       coalesce(dicprevs.name, '')                                    prev_state,
       dics.name                                                      state,
       (ba.ei_address_state_id is not null)                           penalized,
       coalesce(dic_ba.name, '')                                      penalty_reason,
       coalesce(ei.approved, false)                                   approved,
       coalesce(ei.missed, false)                                     missed,
       coalesce(ei.made_flips, 0)                                     made_flips,
       coalesce(ei.available_flips, 0)                                available_flips,
       coalesce(vrs.validation, 0)                                    validation,
       coalesce(vrs.validation_missed, 0)                             validation_missed,
       coalesce(vrs.validation_missed_reason, 0)                      validation_missed_reason,
       coalesce(vrs.flips, 0)                                         flips,
       coalesce(vrs.flips_missed, 0)                                  flips_missed,
       coalesce(vrs.flips_missed_reason, 0)                           flips_missed_reason,
       coalesce(vrs.invitations, 0)                                   invitations,
       coalesce(vrs.invitations_missed, 0)                            invitations_missed,
       coalesce(vrs.invitations_missed_reason, 0)                     invitations_missed_reason,
       coalesce(vrs.reports, 0)                                       reports,
       coalesce(vrs.reports_missed, 0)                                reports_missed,
       coalesce(vrs.reports_missed_reason, 0)                         reports_missed_reason,
       coalesce(vrs.candidate, 0)                                     canidadate,
       coalesce(vrs.candidate_missed, 0)                              canidadate_missed,
       coalesce(vrs.candidate_missed_reason, 0)                       canidadate_missed_reason,
       coalesce(vrs.staking, 0)                                       staking,
       coalesce(vrs.staking_missed, 0)                                staking_missed,
       coalesce(vrs.staking_missed_reason, 0)                         staking_missed_reason,
       coalesce(delegateea.address, '')                               delegatee_address,
       coalesce(dvr.total_balance, 0)                                 delegatee_reward,
       coalesce(rsa.amount, 0)                                        stake
FROM epoch_identities ei
         JOIN address_states s ON s.id = ei.address_state_id AND
                                  s.address_id = (SELECT id FROM addresses WHERE lower(address) = lower($2))
         LEFT JOIN dic_identity_states dics ON dics.id = s.state
         LEFT JOIN address_states prevs ON prevs.id = s.prev_id
         LEFT JOIN dic_identity_states dicprevs ON dicprevs.id = prevs.state
         LEFT JOIN bad_authors ba ON ba.ei_address_state_id = ei.address_state_id
         LEFT JOIN dic_bad_author_reasons dic_ba on dic_ba.id = ba.reason
         LEFT JOIN validation_reward_summaries vrs ON vrs.epoch = ei.epoch AND vrs.address_id = s.address_id
         LEFT JOIN delegatee_validation_rewards dvr ON dvr.epoch = ei.epoch AND dvr.delegator_address_id = s.address_id
         LEFT JOIN addresses delegateea ON delegateea.id = dvr.delegatee_address_id
         LEFT JOIN reward_staked_amounts rsa ON rsa.ei_address_state_id = ei.address_state_id
WHERE ei.epoch = $1