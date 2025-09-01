-- Seed ACL with default allowed subnets
INSERT OR IGNORE INTO acl_subnets (cidr) VALUES 
  ('192.168.10.0/24'),
  ('192.168.11.0/24');
