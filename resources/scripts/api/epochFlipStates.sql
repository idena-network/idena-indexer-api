SELECT dfs.name status, efs.count cnt
FROM epoch_flip_statuses efs
         JOIN dic_flip_statuses dfs ON dfs.id = efs.flip_status
WHERE efs.epoch = $1;