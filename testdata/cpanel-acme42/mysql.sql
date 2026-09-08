CREATE DATABASE IF NOT EXISTS `acme42_wp`;
USE `acme42_wp`;
CREATE TABLE wp_options (
  option_name varchar(64) NOT NULL,
  option_value text
);
INSERT INTO wp_options VALUES ('blogname','Imported Blog');
CREATE DATABASE `acme42_store`;
USE `acme42_store`;
CREATE TABLE products (
  id int NOT NULL,
  name varchar(64) NOT NULL
);
INSERT INTO products VALUES (1,'Widget');
