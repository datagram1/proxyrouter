-- Add Docker subnet to ACL for testing
INSERT OR IGNORE INTO acl_subnets (cidr) VALUES 
  ('192.168.65.0/24'),
  ('172.16.0.0/12'),
  ('10.0.0.0/8');
