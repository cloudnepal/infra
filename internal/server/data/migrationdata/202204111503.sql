PRAGMA foreign_keys=OFF;
BEGIN TRANSACTION;
CREATE TABLE migrations (id VARCHAR(255) PRIMARY KEY);
INSERT INTO migrations VALUES('SCHEMA_INIT');
INSERT INTO migrations VALUES('202203231621');
INSERT INTO migrations VALUES('202203241643');
INSERT INTO migrations VALUES('202203301642');
INSERT INTO migrations VALUES('202203301652');
INSERT INTO migrations VALUES('202203301643');
INSERT INTO migrations VALUES('202203301644');
INSERT INTO migrations VALUES('202203301645');
INSERT INTO migrations VALUES('202203301646');
INSERT INTO migrations VALUES('202203301647');
INSERT INTO migrations VALUES('202203301648');
INSERT INTO migrations VALUES('202204061643');
CREATE TABLE `groups` (`id` integer,`created_at` datetime,`updated_at` datetime,`deleted_at` datetime,`name` text,`provider_id` integer,PRIMARY KEY (`id`));
INSERT INTO "groups" VALUES(36544743275307008,'2022-04-11 20:15:45.765601217+00:00','2022-04-11 20:15:45.765601217+00:00',NULL,'Everyone',36544743262724096);
INSERT INTO "groups" VALUES(36547840424878080,'2022-04-11 20:28:04.189253586+00:00','2022-04-11 20:28:04.189253586+00:00',NULL,'Engineering',0);
CREATE TABLE `identities_groups` (`group_id` integer,`identity_id` integer,PRIMARY KEY (`group_id`,`identity_id`),CONSTRAINT `fk_identities_groups_group` FOREIGN KEY (`group_id`) REFERENCES `groups`(`id`),CONSTRAINT `fk_identities_groups_identity` FOREIGN KEY (`identity_id`) REFERENCES `identities`(`id`));
INSERT INTO identities_groups VALUES(36547840424878080,36547838281588736);
INSERT INTO identities_groups VALUES(36544743275307008,36547838281588736);
CREATE TABLE `grants` (`id` integer,`created_at` datetime,`updated_at` datetime,`deleted_at` datetime,`subject` text,`privilege` text,`resource` text,`created_by` integer,PRIMARY KEY (`id`));
INSERT INTO grants VALUES(36544743275307009,'2022-04-11 20:15:45.765891717+00:00','2022-04-11 20:15:45.765891717+00:00',NULL,'g:5VnbrkDoMw','cluster-admin','kubernetes.docker-desktop',1);
INSERT INTO grants VALUES(36544743279501313,'2022-04-11 20:15:45.766258592+00:00','2022-04-11 20:15:45.766258592+00:00',NULL,'i:5Vnbrm1TB7','cluster-admin','kubernetes.docker-desktop',1);
INSERT INTO grants VALUES(36547838294171648,'2022-04-11 20:28:03.680813086+00:00','2022-04-11 20:28:03.680813086+00:00',NULL,'i:5VozJLKzWG','admin','infra',0);
INSERT INTO grants VALUES(36548620389261312,'2022-04-11 20:31:10.152810048+00:00','2022-04-11 20:31:10.152810048+00:00',NULL,'i:5VozJLKzWH','admin','infra',36547838281588736);
CREATE TABLE `providers` (`id` integer,`created_at` datetime,`updated_at` datetime,`deleted_at` datetime,`name` text,`url` text,`client_id` text,`client_secret` text,PRIMARY KEY (`id`));
INSERT INTO providers VALUES(36544743090757632,'2022-04-11 20:15:45.721963717+00:00','2022-04-11 20:15:45.763277384+00:00',NULL,'infra','','','');
INSERT INTO providers VALUES(36544743262724096,'2022-04-11 20:15:45.762991842+00:00','2022-04-11 20:28:03.681333795+00:00',NULL,'okta','dev.okta.com','','');
CREATE TABLE `provider_tokens` (`id` integer,`created_at` datetime,`updated_at` datetime,`deleted_at` datetime,`user_id` integer,`provider_id` integer,`redirect_url` text,`access_token` text,`refresh_token` text,`expires_at` datetime,PRIMARY KEY (`id`));
CREATE TABLE `destinations` (`id` integer,`created_at` datetime,`updated_at` datetime,`deleted_at` datetime,`name` text,`unique_id` text,`connection_url` text,`connection_ca` text,PRIMARY KEY (`id`));
CREATE TABLE `access_keys` (`id` integer,`created_at` datetime,`updated_at` datetime,`deleted_at` datetime,`name` text,`issued_for` integer,`expires_at` datetime,`extension` integer,`extension_deadline` datetime,`key_id` text,`secret_checksum` blob,PRIMARY KEY (`id`));
CREATE TABLE `settings` (`id` integer,`created_at` datetime,`updated_at` datetime,`deleted_at` datetime,`private_jwk` blob,`public_jwk` blob,`setup_required` numeric,PRIMARY KEY (`id`));
CREATE TABLE `encryption_keys` (`id` integer,`created_at` datetime,`updated_at` datetime,`deleted_at` datetime,`key_id` integer,`name` text,`encrypted` blob,`algorithm` text,`root_key_id` text,PRIMARY KEY (`id`));
CREATE TABLE `trusted_certificates` (`id` integer,`created_at` datetime,`updated_at` datetime,`deleted_at` datetime,`key_algorithm` text,`signing_algorithm` text,`public_key` text,`cert_pem` blob,`identity` text,`expires_at` datetime,`one_time_use` numeric,PRIMARY KEY (`id`));
CREATE TABLE `root_certificates` (`id` integer,`created_at` datetime,`updated_at` datetime,`deleted_at` datetime,`key_algorithm` text,`signing_algorithm` text,`public_key` text,`private_key` text,`signed_cert` text,`expires_at` datetime,PRIMARY KEY (`id`));
CREATE TABLE `credentials` (`id` integer,`created_at` datetime,`updated_at` datetime,`deleted_at` datetime,`identity_id` integer,`password_hash` blob,`one_time_password` numeric,`one_time_password_used` numeric,PRIMARY KEY (`id`));
CREATE TABLE IF NOT EXISTS "identities" (`id` integer,`created_at` datetime,`updated_at` datetime,`deleted_at` datetime,`kind` text,`name` text,`last_seen_at` datetime,`provider_id` integer,PRIMARY KEY (`id`),CONSTRAINT `fk_providers_users` FOREIGN KEY (`provider_id`) REFERENCES `providers`(`id`));
INSERT INTO identities VALUES(36544743279501312,'2022-04-11 20:15:45.766136925+00:00','2022-04-11 20:15:45.766136925+00:00',NULL,'machine','admin','0001-01-01 00:00:00+00:00',36544743090757632);
INSERT INTO identities VALUES(36547838281588736,'2022-04-11 20:28:03.677883545+00:00','2022-04-11 20:31:34.288667837+00:00',NULL,'user','steven@example.com','2022-04-11 20:31:34.287892045+00:00',36544743262724096);
INSERT INTO identities VALUES(36547838281588737,NULL,NULL,NULL,'user','steven@example.com',NULL,36544743090757632);
CREATE UNIQUE INDEX `idx_groups_name_provider_id` ON `groups`(`name`,`provider_id`) WHERE deleted_at is NULL;
CREATE UNIQUE INDEX `idx_providers_name` ON `providers`(`name`) WHERE deleted_at is NULL;
CREATE UNIQUE INDEX `idx_destinations_unique_id` ON `destinations`(`unique_id`) WHERE deleted_at is NULL;
CREATE UNIQUE INDEX `idx_access_keys_key_id` ON `access_keys`(`key_id`) WHERE deleted_at is NULL;
CREATE UNIQUE INDEX `idx_access_keys_name` ON `access_keys`(`name`) WHERE deleted_at is NULL;
CREATE UNIQUE INDEX `idx_encryption_keys_key_id` ON `encryption_keys`(`key_id`);
CREATE UNIQUE INDEX `idx_credentials_identity_id` ON `credentials`(`identity_id`) WHERE deleted_at is NULL;
COMMIT;