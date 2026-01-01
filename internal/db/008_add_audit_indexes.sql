-- Add indexes to cw_audit_logs table for better query performance

-- Create composite index for user and creation time (common for user's own history)
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_created ON cw_audit_logs (user_id, created_at);

-- Create composite index for action and creation time (filtering by action type)
CREATE INDEX IF NOT EXISTS idx_audit_logs_action_created ON cw_audit_logs (action, created_at);

-- Create composite index for resource and ID (tracking specific resource history)
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_id ON cw_audit_logs (resource, resource_id);

-- Create index for result status (filtering failures)
CREATE INDEX IF NOT EXISTS idx_audit_logs_success_created ON cw_audit_logs (success, created_at);
