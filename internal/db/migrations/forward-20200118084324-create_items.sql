CREATE TABLE IF NOT EXISTS `items` (
	`uuid` varchar(36) NOT NULL,
	`user_uuid` varchar(36) NOT NULL,
    `content` longblob NOT NULL,
    `content_type` varchar(255) NOT NULL,
    `enc_item_key` varchar(1024) NOT NULL,
    `auth_hash` varchar(1024) NOT NULL,
    `deleted` tinyint(1) NOT NULL DEFAULT 0,
    `created_at` datetime(6) DEFAULT CURRENT_TIMESTAMP NOT NULL,
    `updated_at` datetime(6) DEFAULT CURRENT_TIMESTAMP NOT NULL,
	PRIMARY KEY (`uuid`),
	KEY `index_items_on_user_uuid_and_content_type` (`user_uuid`, `content_type`) USING BTREE,
	KEY `index_items_on_user_uuid_and_updated_at` (`user_uuid`, `updated_at`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
