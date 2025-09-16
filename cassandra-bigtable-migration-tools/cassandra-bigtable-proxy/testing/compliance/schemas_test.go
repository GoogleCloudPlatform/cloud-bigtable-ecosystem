/*
 * Copyright (C) 2025 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy of
 * the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
 * WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
 * License for the specific language governing permissions and limitations under
 * the License.
 */
package compliance

// add test tables that are used by multiple tests here
// note: some tests, typically those that test DDL commands, create and manage their own tables, separately, at runtime
func getSchemas() []string {
	schemas := []string{
		`
CREATE TABLE IF NOT EXISTS bigtabledevinstance.user_info (
	name text,
	age bigint,
	code int,
	credited double,
	text_col text,
	balance float,
	is_active boolean,
	birth_date timestamp,
	zip_code bigint,
	extra_info map<text,text>,
	map_text_int map<text,int>,
	map_text_bigint map<text,bigint>,
	map_text_boolean map<text,boolean>,
	map_text_ts map<text,timestamp>,
	map_text_float map<text,float>,
	map_text_double map<text,double>,
	ts_text_map map<timestamp,text>,
	ts_boolean_map map<timestamp,boolean>,
	ts_float_map map<timestamp,float>,
	ts_double_map map<timestamp,double>,
	ts_bigint_map map<timestamp,bigint>,
	ts_ts_map map<timestamp,timestamp>,
	ts_int_map map<timestamp,int>,
	tags set<text>,
	set_boolean set<boolean>,
	set_int set<int>,
	set_bigint set<bigint>,
	set_float set<float>,
	set_double set<double>,
	set_timestamp set<timestamp>,
	list_text list<text>,
	list_int list<int>,
	list_bigint list<bigint>,
	list_float list<float>,
	list_double list<double>,
	list_boolean list<boolean>,
	list_timestamp list<timestamp>,
	PRIMARY KEY (age, name)
);
`,

		`
CREATE TABLE IF NOT EXISTS bigtabledevinstance.orders (
	user_id varchar,
	order_num int,
	name varchar,
	PRIMARY KEY (user_id, order_num)
);
`,

		`
CREATE TABLE IF NOT EXISTS bigtabledevinstance.aggregation_grouping_test (
	region text,
	category varchar,
	item_id int,
	sale_timestamp timestamp,
	quantity int,
	price float,
	discount double,
	revenue_bigint bigint,
	PRIMARY KEY (category, item_id)
);
`,

		`
CREATE TABLE IF NOT EXISTS bigtabledevinstance.multiple_int_keys (
	user_id bigint,
	order_num int,
	name varchar,
	PRIMARY KEY (user_id, order_num)
);
`,
		`
CREATE TABLE IF NOT EXISTS bigtabledevinstance.test_int_key (
	user_id bigint PRIMARY KEY,
	name varchar
);
`,
		`
CREATE TABLE IF NOT EXISTS bigtabledevinstance.social_posts (
	user_id varchar,
	id int,
	likes counter,
	views counter,
	PRIMARY KEY (user_id, id)
);
`,
		`
CREATE TABLE IF NOT EXISTS bigtabledevinstance.frozen_table (
	name varchar,
	list_nums frozen<list<int>>,
	map_text frozen<map<text, text>>,
	set_text frozen<set<text>>,
	PRIMARY KEY (name)
);
`,
	}

	if testTarget == TestTargetProxy {
		schemas = append(schemas,
			// this table is only for testing bigtable big endian support so no need to test with cassandra - the custom 'int_row_key_encoding' will likely be rejected by cassandra
			`
CREATE TABLE IF NOT EXISTS bigtabledevinstance.orders_big_endian_encoded (
	user_id varchar,
	order_num int,
	name varchar,
	PRIMARY KEY (user_id, order_num)
) WITH int_row_key_encoding='big_endian';
`,
		)
	}

	return schemas
}
