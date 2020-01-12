CREATE TABLE IF NOT EXISTS `users` (
	`uuid` varchar(36) NOT NULL,
	`email` varchar(255) NOT NULL,
	`password` varchar(255) NOT NULL,
	`pw_func` varchar(255) NOT NULL DEFAULT "pbkdf2",
	`pw_alg` varchar(255) NOT NULL DEFAULT "sha512",
	`pw_cost` integer NOT NULL DEFAULT 110000,
	`pw_key_size` integer NOT NULL DEFAULT 512,
	`pw_nonce` varchar(255) NOT NULL,
	`pw_salt` varchar(255) NOT NULL,
	`locked_until` datetime(6) DEFAULT CURRENT_TIMESTAMP NOT NULL,
	`created_at` datetime(6) DEFAULT CURRENT_TIMESTAMP NOT NULL,
	`updated_at` datetime(6) DEFAULT CURRENT_TIMESTAMP NOT NULL,
	`num_failed_attempts` int(11) DEFAULT 0 NOT NULL,
	PRIMARY KEY (`uuid`),
	KEY `index_users_on_email` (`email`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
