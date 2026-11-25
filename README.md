# snowflake-krm-extensions

Kubernetes kinds for the Snowflake configuration that Terraform's
`snowflakedb/snowflake` provider manages and Snowflake's operators do not. Snowflake publishes no Kubernetes operator for account objects (databases, warehouses, roles, grants).

By the numbers: `snowflakedb/snowflake` is the #42 Terraform provider by downloads
(88,504,671); version 2.23.0 has 154 resources. Operators
cover 0. This repository adds **112 kinds** standing in for
**153 more**. Each gap kind is listed below with its Terraform equivalent.

**Limits.** Roles, accounts and managed (reader) accounts are read by listing (the REST API has no get-one for them) and grants are links: an AccountRoleGrant is applied with POST /roles/{name}/grants and removed with grants:revoke. Snowflake uppercases unquoted names, so case alone is not drift. Deleting an `Account` object drops the account with a 90-day grace period (`gracePeriodInDays=90`, the longest the API allows), in which an ORGADMIN can still restore it (`UNDROP ACCOUNT`). What the REST API has no resource for, 71 kinds manage with SQL instead (see "SQL kinds" below).

**Kinds Snowflake cannot update.** 26 kinds have no update in the REST API (no `ALTER`). By default a changed spec is reported, not applied: the object goes `Ready=False` (`SyncFailed`, "cannot be updated in place") and nothing is sent until the spec matches again or you delete and recreate the object. Twelve kinds that hold definitions, not data, may opt in, per object, to applying a change with `CREATE OR REPLACE` (the create call again with `createMode=orReplace`):

```yaml
spec:
  replaceOnChange: true
```

| May opt in | Never (holds data) | Report drift only (state or identity) |
| --- | --- | --- |
| `Alert`, `CodeBundle`, `Function`, `NetworkPolicy`, `NetworkRule`, `Notebook`, `PasswordPolicy`, `Pipe`, `Procedure`, `Stream`, `Streamlit`, `View` | `Table`, `DynamicTable`, `IcebergTable`, `Stage`, `ExternalVolume`, `Account`, `ManagedAccount` | `AccountRole`, `DatabaseRole`, `Secret`, `Sequence`, `CatalogIntegration`, `NotificationIntegration`, `ImageRepository`, `CortexSearchService` |

The last column stays report-only because replacing would drop the role's grants, the secret's value, the sequence's position, the integration's trust or external IDs, the repository's images, or the search service's index (SFREPL-1, builder, 2026-11-15).

The allowlist is `replaceOnChange` in `hack/gen/vendor.yaml`; only those kinds' CRDs have the field, so the API server does not accept it on any other kind, and the controller refuses it too. `Task` is stateless as well but is updated in place, so it needs no opt-in. Replacing is not free: a replaced object loses what Snowflake keeps on it. A stream starts again from the current table version (its offset is lost), an alert is created suspended again, a pipe loses its load history (files it loaded in the last 14 days can be loaded again), and grants on the object are dropped unless they are re-applied (an `AccountRoleGrant` or `DatabaseRoleGrant` object is, on its next resync). A change to the name, or to a function's or procedure's name or arguments, is never applied by replacing, since that would make a second object and leave the first: delete and recreate the object for that. The first create never replaces, so an existing object of the same name is not overwritten.

**Readiness: proven against a fake Snowflake API only.** Every kind passes
create, update, drift repair and delete against a fake built from the same
descriptors (`TestEveryKindOnFake`). None has run against the real API yet;
treat each as untested there until it has.

## How it works

The kinds are generated, not hand-written. `hack/gen/vendor.yaml` pins
Snowflake's OpenAPI spec; [krm-rest-runtime](https://github.com/highfivelabs/krm-rest-runtime)'s
generator finds every resource the API exposes in a create/read/update/delete
shape, keeps those that stand in for a Terraform resource in the gap list
(`hack/gen/terraform-gaps.json`), and writes:

- `types.yaml`: per kind, its API calls, path parameters and Terraform equivalents
- `config/crds/`: one CRD per kind, its spec the API's request body
- `zz_generated.report.md`: the full list

One generic manager (`cmd/manager`) runs every kind on krm-rest-runtime's
reconciler: it creates the resource, reads it back every 10 minutes and
puts back anything changed outside Kubernetes, and deletes it with the
object. Spec fields keep Snowflake's own names, so the API reference
applies field for field. Path parameters are spec fields too.

Hand corrections to the automatic matching (a Terraform type matched to
the wrong API path, or not matched) are in `vendor.yaml`'s `overrides`.

## Install

```sh
kubectl kustomize config/default | KO_DOCKER_REPO=<registry> ko resolve -f - | kubectl apply --server-side -f -
```

Then, in each namespace that manages Snowflake, a Secret with
`token` and a Connection named `default`
(`config/samples/connection.yaml`). Objects use it unless
`spec.connectionRef.name` names another. Secret values (tokens, passwords,
secret values) go in a Kubernetes Secret, not the spec:
`spec.secretRefs: {<field>: {name, key}}` fills that body field.

To run only some API groups, install those groups' CRDs from `config/crds`
and pass `--groups` to the manager.

## Kinds

| Kind | Terraform equivalent | Shape | Read |
| --- | --- | --- | --- |
| `account.snowflake.hfl.dev/v1alpha1` `Account` | `snowflake_account` | crud | `GET /api/v2/accounts` |
| `account.snowflake.hfl.dev/v1alpha1` `AccountAuthenticationPolicyAttachment` | `snowflake_account_authentication_policy_attachment` | sql (set) | `SQL: ALTER ACCOUNT SET AUTHENTICATION` |
| `account.snowflake.hfl.dev/v1alpha1` `AccountParameter` | `snowflake_account_parameter` | sql (set) | `SQL: ALTER ACCOUNT SET` |
| `account.snowflake.hfl.dev/v1alpha1` `AccountPasswordPolicyAttachment` | `snowflake_account_password_policy_attachment` | sql (set) | `SQL: ALTER ACCOUNT SET PASSWORD` |
| `account.snowflake.hfl.dev/v1alpha1` `AccountRole` | `snowflake_account_role` | crud | `GET /api/v2/roles` |
| `account.snowflake.hfl.dev/v1alpha1` `AccountRoleGrant` | `snowflake_grant_privileges_to_account_role` | assoc | `GET /api/v2/roles/{name}/grants` |
| `account.snowflake.hfl.dev/v1alpha1` `AccountSessionPolicyAttachment` | `snowflake_account_session_policy_attachment` | sql (set) | `SQL: ALTER ACCOUNT SET SESSION` |
| `alert.snowflake.hfl.dev/v1alpha1` `Alert` | `snowflake_alert` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/alerts/{name}` |
| `api.snowflake.hfl.dev/v1alpha1` `ApiAuthenticationIntegration` | `snowflake_api_authentication_integration_with_authorization_code_grant`, `snowflake_api_authentication_integration_with_client_credentials`, `snowflake_api_authentication_integration_with_jwt_bearer` | sql (object) | `SQL: CREATE SECURITY INTEGRATION` |
| `api.snowflake.hfl.dev/v1alpha1` `ApiIntegration` | `snowflake_api_integration`, `snowflake_api_integration_amazon_api_gateway`, `snowflake_api_integration_azure_api_management`, `snowflake_api_integration_git_repository_token`, `snowflake_api_integration_google_cloud_api_gateway` | crud | `GET /api/v2/api-integrations/{name}` |
| `api.snowflake.hfl.dev/v1alpha1` `ApiIntegrationExternalMcpDynamicClient` | `snowflake_api_integration_external_mcp_dynamic_client` | sql (object) | `SQL: CREATE API INTEGRATION` |
| `api.snowflake.hfl.dev/v1alpha1` `ApiIntegrationExternalMcpOauth2` | `snowflake_api_integration_external_mcp_oauth2` | sql (object) | `SQL: CREATE API INTEGRATION` |
| `api.snowflake.hfl.dev/v1alpha1` `ApiIntegrationGitRepositoryGithubApp` | `snowflake_api_integration_git_repository_github_app` | sql (object) | `SQL: CREATE API INTEGRATION` |
| `api.snowflake.hfl.dev/v1alpha1` `ApiIntegrationGitRepositoryOauth2` | `snowflake_api_integration_git_repository_oauth2` | sql (object) | `SQL: CREATE API INTEGRATION` |
| `api.snowflake.hfl.dev/v1alpha1` `ApiIntegrationGitRepositoryPrivateLink` | `snowflake_api_integration_git_repository_private_link` | sql (object) | `SQL: CREATE API INTEGRATION` |
| `authentication.snowflake.hfl.dev/v1alpha1` `AuthenticationPolicy` | `snowflake_authentication_policy` | sql (object) | `SQL: CREATE AUTHENTICATION POLICY` |
| `backup.snowflake.hfl.dev/v1alpha1` `BackupPolicy` | `snowflake_backup_policy` | sql (object) | `SQL: CREATE BACKUP POLICY` |
| `catalog.snowflake.hfl.dev/v1alpha1` `CatalogIntegration` | `snowflake_catalog_integration_aws_glue`, `snowflake_catalog_integration_iceberg_rest`, `snowflake_catalog_integration_object_storage`, `snowflake_catalog_integration_open_catalog` | crud | `GET /api/v2/catalog-integrations/{name}` |
| `code.snowflake.hfl.dev/v1alpha1` `CodeBundle` | `snowflake_code_bundle` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/code-bundles/{name}` |
| `compute.snowflake.hfl.dev/v1alpha1` `ComputePool` | `snowflake_compute_pool` | crud | `GET /api/v2/compute-pools/{name}` |
| `connection.snowflake.hfl.dev/v1alpha1` `PrimaryConnection` | `snowflake_primary_connection` | sql (object) | `SQL: CREATE CONNECTION COMMENT` |
| `connection.snowflake.hfl.dev/v1alpha1` `SecondaryConnection` | `snowflake_secondary_connection` | sql (object) | `SQL: CREATE CONNECTION AS` |
| `cortex.snowflake.hfl.dev/v1alpha1` `CortexAgent` | `snowflake_cortex_agent` | sql (object) | `SQL: CREATE AGENT COMMENT` |
| `cortex.snowflake.hfl.dev/v1alpha1` `CortexSearchService` | `snowflake_cortex_search_service` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/cortex-search-services/{name}` |
| `current.snowflake.hfl.dev/v1alpha1` `CurrentAccount` | `snowflake_current_account` | sql (set, field by field) | `SQL: ALTER ACCOUNT SET` |
| `current.snowflake.hfl.dev/v1alpha1` `CurrentOrganizationAccount` | `snowflake_current_organization_account` | sql (set, field by field) | `SQL: ALTER ORGANIZATION ACCOUNT SET` |
| `database.snowflake.hfl.dev/v1alpha1` `Database` | `snowflake_database` | crud | `GET /api/v2/databases/{name}` |
| `database.snowflake.hfl.dev/v1alpha1` `DatabaseRole` | `snowflake_database_role` | crud | `GET /api/v2/databases/{database}/database-roles` |
| `database.snowflake.hfl.dev/v1alpha1` `DatabaseRoleGrant` | `snowflake_grant_database_role`, `snowflake_grant_privileges_to_database_role` | assoc | `GET /api/v2/databases/{database}/database-roles/{name}/grants` |
| `database.snowflake.hfl.dev/v1alpha1` `SecondaryDatabase` | `snowflake_secondary_database` | sql (object) | `SQL: CREATE DATABASE AS` |
| `database.snowflake.hfl.dev/v1alpha1` `SharedDatabase` | `snowflake_shared_database` | sql (object) | `SQL: CREATE DATABASE FROM` |
| `dynamic.snowflake.hfl.dev/v1alpha1` `DynamicTable` | `snowflake_dynamic_table` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/dynamic-tables/{name}` |
| `external.snowflake.hfl.dev/v1alpha1` `ExternalAccessIntegration` | `snowflake_external_access_integration` | sql (object) | `SQL: CREATE EXTERNAL ACCESS INTEGRATION` |
| `external.snowflake.hfl.dev/v1alpha1` `ExternalFunction` | `snowflake_external_function` | sql (object) | `SQL: CREATE EXTERNAL FUNCTION` |
| `external.snowflake.hfl.dev/v1alpha1` `ExternalOauthIntegration` | `snowflake_external_oauth_integration` | sql (object) | `SQL: CREATE SECURITY INTEGRATION` |
| `external.snowflake.hfl.dev/v1alpha1` `ExternalTable` | `snowflake_external_table` | sql (object) | `SQL: CREATE EXTERNAL TABLE` |
| `external.snowflake.hfl.dev/v1alpha1` `ExternalVolume` | `snowflake_external_volume` | crud | `GET /api/v2/external-volumes/{name}` |
| `failover.snowflake.hfl.dev/v1alpha1` `FailoverGroup` | `snowflake_failover_group` | sql (object) | `SQL: CREATE FAILOVER GROUP` |
| `file.snowflake.hfl.dev/v1alpha1` `FileFormat` | `snowflake_file_format`, `snowflake_file_format_avro`, `snowflake_file_format_csv`, `snowflake_file_format_json`, `snowflake_file_format_orc`, `snowflake_file_format_parquet`, `snowflake_file_format_xml` | sql (object) | `SQL: CREATE FILE FORMAT` |
| `function.snowflake.hfl.dev/v1alpha1` `Function` | `snowflake_function_java`, `snowflake_function_javascript`, `snowflake_function_python`, `snowflake_function_scala`, `snowflake_function_sql` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/user-defined-functions/{nameWithArgs}` |
| `git.snowflake.hfl.dev/v1alpha1` `GitRepository` | `snowflake_git_repository` | sql (object) | `SQL: CREATE GIT REPOSITORY` |
| `grant.snowflake.hfl.dev/v1alpha1` `GrantApplicationRole` | `snowflake_grant_application_role` | sql (grant) | `SQL: GRANT APPLICATION ROLE` |
| `grant.snowflake.hfl.dev/v1alpha1` `GrantOwnership` | `snowflake_grant_ownership` | sql (set) | `SQL: GRANT OWNERSHIP ON` |
| `grant.snowflake.hfl.dev/v1alpha1` `GrantPrivilegesToShare` | `snowflake_grant_privileges_to_share` | sql (grant) | `SQL: GRANT ON` |
| `hybrid.snowflake.hfl.dev/v1alpha1` `HybridTable` | `snowflake_hybrid_table` | sql (object) | `SQL: CREATE HYBRID TABLE` |
| `iceberg.snowflake.hfl.dev/v1alpha1` `IcebergTable` | `snowflake_iceberg_table` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/iceberg-tables/{name}` |
| `iceberg.snowflake.hfl.dev/v1alpha1` `IcebergTableFromAwsGlue` | `snowflake_iceberg_table_from_aws_glue` | sql (object) | `SQL: CREATE ICEBERG TABLE` |
| `iceberg.snowflake.hfl.dev/v1alpha1` `IcebergTableFromDeltaFiles` | `snowflake_iceberg_table_from_delta_files` | sql (object) | `SQL: CREATE ICEBERG TABLE` |
| `iceberg.snowflake.hfl.dev/v1alpha1` `IcebergTableFromFiles` | `snowflake_iceberg_table_from_files` | sql (object) | `SQL: CREATE ICEBERG TABLE` |
| `iceberg.snowflake.hfl.dev/v1alpha1` `IcebergTableFromRest` | `snowflake_iceberg_table_from_rest` | sql (object) | `SQL: CREATE ICEBERG TABLE` |
| `image.snowflake.hfl.dev/v1alpha1` `ImageRepository` | `snowflake_image_repository` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/image-repositories/{name}` |
| `intelligence.snowflake.hfl.dev/v1alpha1` `IntelligenceCortexAgentAttachment` | `snowflake_intelligence_cortex_agent_attachment` | sql (grant) | `SQL: ALTER SNOWFLAKE INTELLIGENCE` |
| `job.snowflake.hfl.dev/v1alpha1` `JobService` | `snowflake_job_service` | sql (object) | `SQL: EXECUTE JOB SERVICE IN` |
| `listing.snowflake.hfl.dev/v1alpha1` `Listing` | `snowflake_listing` | sql (object) | `SQL: CREATE EXTERNAL LISTING` |
| `managed.snowflake.hfl.dev/v1alpha1` `ManagedAccount` | `snowflake_managed_account` | crud | `GET /api/v2/managed-accounts` |
| `masking.snowflake.hfl.dev/v1alpha1` `MaskingPolicy` | `snowflake_masking_policy` | sql (object) | `SQL: CREATE MASKING POLICY` |
| `materialized.snowflake.hfl.dev/v1alpha1` `MaterializedView` | `snowflake_materialized_view` | sql (object) | `SQL: MATERIALIZED VIEW` |
| `mcp.snowflake.hfl.dev/v1alpha1` `McpServer` | `snowflake_mcp_server` | sql (object) | `SQL: CREATE MCP SERVER` |
| `network.snowflake.hfl.dev/v1alpha1` `NetworkPolicy` | `snowflake_network_policy` | crud | `GET /api/v2/network-policies/{name}` |
| `network.snowflake.hfl.dev/v1alpha1` `NetworkPolicyAttachment` | `snowflake_network_policy_attachment` | sql (set) | `SQL: ALTER USER SET` |
| `network.snowflake.hfl.dev/v1alpha1` `NetworkRule` | `snowflake_network_rule` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/network-rules/{name}` |
| `notebook.snowflake.hfl.dev/v1alpha1` `Notebook` | `snowflake_notebook` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/notebooks/{name}` |
| `notification.snowflake.hfl.dev/v1alpha1` `NotificationIntegration` | `snowflake_email_notification_integration`, `snowflake_notification_integration` | crud | `GET /api/v2/notification-integrations/{name}` |
| `oauth.snowflake.hfl.dev/v1alpha1` `OauthIntegrationForCustomClients` | `snowflake_oauth_integration_for_custom_clients` | sql (object) | `SQL: CREATE SECURITY INTEGRATION` |
| `oauth.snowflake.hfl.dev/v1alpha1` `OauthIntegrationForPartnerApplications` | `snowflake_oauth_integration_for_partner_applications` | sql (object) | `SQL: CREATE SECURITY INTEGRATION` |
| `object.snowflake.hfl.dev/v1alpha1` `ObjectParameter` | `snowflake_object_parameter` | sql (set) | `SQL: ALTER SET` |
| `openflow.snowflake.hfl.dev/v1alpha1` `OpenflowConnector` | `snowflake_openflow_connector` | sql (object) | `SQL: CREATE OPENFLOW CONNECTOR` |
| `openflow.snowflake.hfl.dev/v1alpha1` `OpenflowDeploymentByoc` | `snowflake_openflow_deployment_byoc` | sql (object) | `SQL: CREATE OPENFLOW DEPLOYMENT` |
| `openflow.snowflake.hfl.dev/v1alpha1` `OpenflowDeploymentSnowflakeManaged` | `snowflake_openflow_deployment_snowflake_managed` | sql (object) | `SQL: CREATE OPENFLOW DEPLOYMENT` |
| `openflow.snowflake.hfl.dev/v1alpha1` `OpenflowRuntime` | `snowflake_openflow_runtime` | sql (object) | `SQL: CREATE OPENFLOW RUNTIME` |
| `password.snowflake.hfl.dev/v1alpha1` `PasswordPolicy` | `snowflake_password_policy` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/password-policies/{name}` |
| `pipe.snowflake.hfl.dev/v1alpha1` `Pipe` | `snowflake_pipe` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/pipes/{name}` |
| `postgres.snowflake.hfl.dev/v1alpha1` `PostgresInstance` | `snowflake_postgres_instance` | sql (object) | `SQL: CREATE POSTGRES INSTANCE` |
| `postgres.snowflake.hfl.dev/v1alpha1` `PostgresMirror` | `snowflake_postgres_mirror` | sql (object) | `SQL: CALL` |
| `procedure.snowflake.hfl.dev/v1alpha1` `Procedure` | `snowflake_procedure_java`, `snowflake_procedure_javascript`, `snowflake_procedure_python`, `snowflake_procedure_scala`, `snowflake_procedure_sql` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/procedures/{nameWithArgs}` |
| `projection.snowflake.hfl.dev/v1alpha1` `ProjectionPolicy` | `snowflake_projection_policy` | sql (object) | `SQL: CREATE PROJECTION POLICY` |
| `replication.snowflake.hfl.dev/v1alpha1` `ReplicationGroup` | `snowflake_replication_group` | sql (object) | `SQL: CREATE REPLICATION GROUP` |
| `replication.snowflake.hfl.dev/v1alpha1` `ReplicationGroupSecondary` | `snowflake_replication_group_secondary` | sql (object) | `SQL: CREATE REPLICATION GROUP` |
| `resource.snowflake.hfl.dev/v1alpha1` `ResourceMonitor` | `snowflake_resource_monitor` | sql (object) | `SQL: CREATE RESOURCE MONITOR` |
| `row.snowflake.hfl.dev/v1alpha1` `RowAccessPolicy` | `snowflake_row_access_policy` | sql (object) | `SQL: CREATE ROW ACCESS POLICY` |
| `saml2.snowflake.hfl.dev/v1alpha1` `Saml2Integration` | `snowflake_saml2_integration` | sql (object) | `SQL: CREATE SECURITY INTEGRATION` |
| `schema.snowflake.hfl.dev/v1alpha1` `Schema` | `snowflake_schema` | crud | `GET /api/v2/databases/{database}/schemas/{name}` |
| `scim.snowflake.hfl.dev/v1alpha1` `ScimIntegration` | `snowflake_scim_integration` | sql (object) | `SQL: CREATE SECURITY INTEGRATION` |
| `secret.snowflake.hfl.dev/v1alpha1` `Secret` | `snowflake_secret_with_authorization_code_grant`, `snowflake_secret_with_basic_authentication`, `snowflake_secret_with_client_credentials`, `snowflake_secret_with_generic_string` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/secrets/{name}` |
| `semantic.snowflake.hfl.dev/v1alpha1` `SemanticView` | `snowflake_semantic_view` | sql (object) | `SQL: CREATE SEMANTIC VIEW` |
| `sequence.snowflake.hfl.dev/v1alpha1` `Sequence` | `snowflake_sequence` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/sequences/{name}` |
| `service.snowflake.hfl.dev/v1alpha1` `Service` | `snowflake_service` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/services/{name}` |
| `session.snowflake.hfl.dev/v1alpha1` `SessionPolicy` | `snowflake_session_policy` | sql (object) | `SQL: CREATE SESSION POLICY` |
| `share.snowflake.hfl.dev/v1alpha1` `Share` | `snowflake_share` | sql (object) | `SQL: CREATE SHARE SECURE_OBJECTS_ONLY` |
| `stage.snowflake.hfl.dev/v1alpha1` `Stage` | `snowflake_stage`, `snowflake_stage_external_azure`, `snowflake_stage_external_gcs`, `snowflake_stage_external_s3`, `snowflake_stage_external_s3_compatible`, `snowflake_stage_internal` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/stages/{name}` |
| `storage.snowflake.hfl.dev/v1alpha1` `StorageIntegration` | `snowflake_storage_integration`, `snowflake_storage_integration_aws`, `snowflake_storage_integration_azure`, `snowflake_storage_integration_gcs` | sql (object) | `SQL: CREATE STORAGE INTEGRATION` |
| `storage.snowflake.hfl.dev/v1alpha1` `StorageLifecyclePolicy` | `snowflake_storage_lifecycle_policy` | sql (object) | `SQL: CREATE STORAGE LIFECYCLE POLICY` |
| `stream.snowflake.hfl.dev/v1alpha1` `Stream` | `snowflake_stream_on_directory_table`, `snowflake_stream_on_external_table`, `snowflake_stream_on_table`, `snowflake_stream_on_view` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/streams/{name}` |
| `streamlit.snowflake.hfl.dev/v1alpha1` `Streamlit` | `snowflake_streamlit` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/streamlits/{name}` |
| `table.snowflake.hfl.dev/v1alpha1` `Table` | `snowflake_table` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/tables/{name}` |
| `table.snowflake.hfl.dev/v1alpha1` `TableColumnMaskingPolicyApplication` | `snowflake_table_column_masking_policy_application` | sql (set) | `SQL: ALTER TABLE MODIFY` |
| `table.snowflake.hfl.dev/v1alpha1` `TableConstraint` | `snowflake_table_constraint` | sql (grant) | `SQL: ALTER TABLE ADD` |
| `table.snowflake.hfl.dev/v1alpha1` `TableStorageLifecyclePolicyAttachment` | `snowflake_table_storage_lifecycle_policy_attachment` | sql (grant) | `SQL: ALTER ADD` |
| `tag.snowflake.hfl.dev/v1alpha1` `Tag` | `snowflake_tag` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/tags/{name}` |
| `tag.snowflake.hfl.dev/v1alpha1` `TagAssociation` | `snowflake_tag_association` | sql (set) | `SQL: ALTER SET` |
| `task.snowflake.hfl.dev/v1alpha1` `Task` | `snowflake_task` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/tasks/{name}` |
| `user.snowflake.hfl.dev/v1alpha1` `User` | `snowflake_legacy_service_user`, `snowflake_service_user`, `snowflake_user` | crud | `GET /api/v2/users/{name}` |
| `user.snowflake.hfl.dev/v1alpha1` `UserAuthenticationPolicyAttachment` | `snowflake_user_authentication_policy_attachment` | sql (set) | `SQL: ALTER USER SET` |
| `user.snowflake.hfl.dev/v1alpha1` `UserPasswordPolicyAttachment` | `snowflake_user_password_policy_attachment` | sql (set) | `SQL: ALTER USER SET` |
| `user.snowflake.hfl.dev/v1alpha1` `UserProgrammaticAccessToken` | `snowflake_user_programmatic_access_token` | sql (object) | `SQL: ALTER USER ADD` |
| `user.snowflake.hfl.dev/v1alpha1` `UserPublicKeySet` | `snowflake_user_public_keys` | sql (set) | `SQL: ALTER USER SET` |
| `user.snowflake.hfl.dev/v1alpha1` `UserRoleGrant` | `snowflake_grant_account_role` | assoc | `GET /api/v2/users/{name}/grants` |
| `user.snowflake.hfl.dev/v1alpha1` `UserSessionPolicyAttachment` | `snowflake_user_session_policy_attachment` | sql (set) | `SQL: ALTER USER SET` |
| `view.snowflake.hfl.dev/v1alpha1` `View` | `snowflake_view` | crud | `GET /api/v2/databases/{database}/schemas/{schema}/views/{name}` |
| `warehouse.snowflake.hfl.dev/v1alpha1` `Warehouse` | `snowflake_warehouse` | crud | `GET /api/v2/warehouses/{name}` |
| `warehouse.snowflake.hfl.dev/v1alpha1` `WarehouseAdaptive` | `snowflake_warehouse_adaptive` | sql (object) | `SQL: CREATE WAREHOUSE WAREHOUSE_TYPE` |
| `warehouse.snowflake.hfl.dev/v1alpha1` `WarehouseInteractive` | `snowflake_warehouse_interactive` | sql (object) | `SQL: CREATE INTERACTIVE WAREHOUSE` |

Some kinds stand in for several Terraform types that Snowflake's REST API
treats as one object, so a field one variant lacks is still in the CRD:
`ApiIntegration` (`api_hook.type` AWS, AZURE, GC or GIT), `NotificationIntegration`
(`notification_hook.type` EMAIL), `Stage` (internal or external by `url`,
`storage_integration`, `credentials`, `endpoint`; the REST stage has no
access-point ARN or private-link setting) and `User` (`type` SERVICE or
LEGACY_SERVICE).

### SQL kinds

71 kinds stand in for Terraform types Snowflake's REST API has no resource
for (or, for a few, has one only on another call than its kind's). Like
Terraform's provider, they use SQL: each statement goes to the SQL API
(`POST /api/v2/statements`, the same host, token and token type as the REST
kinds), one at a time. Their statements are in hand-written sources,
`hack/gen/sql/*.yaml`, each taken from terraform-provider-snowflake's SDK at
2.23.0 (the file and option struct are named in the source's header), and
generated into `types.yaml` like the rest. krm-rest-runtime's README ("SQL
kinds") says how values are written: no spec value reaches SQL as written.

- **Objects** (CREATE, SHOW/DESCRIBE, ALTER per field, DROP):
  `MaskingPolicy`, `RowAccessPolicy`, `ProjectionPolicy`, `SessionPolicy`,
  `AuthenticationPolicy`, `StorageLifecyclePolicy`, `BackupPolicy`,
  `ResourceMonitor`, `FileFormat` (Terraform's generic and six typed file
  formats: `type`), `StorageIntegration` (S3, GCS, Azure: `storage_provider`),
  `ExternalAccessIntegration`, `ApiAuthenticationIntegration` (the three
  OAuth flows: `oauth_grant`), `Saml2Integration`, `ScimIntegration`,
  `OauthIntegrationForCustomClients`, `OauthIntegrationForPartnerApplications`,
  `ExternalOauthIntegration`, the five `ApiIntegration*` kinds for the API
  integrations whose `API_USER_AUTHENTICATION` the REST `ApiIntegration` cannot
  hold, `GitRepository`, `Share`, `SecondaryDatabase`, `SharedDatabase`,
  `PrimaryConnection`, `SecondaryConnection`, `MaterializedView`,
  `HybridTable`, `ExternalTable`, `ExternalFunction`, `SemanticView`,
  `CortexAgent`, `McpServer`, `Listing`, `FailoverGroup`, `ReplicationGroup`,
  `ReplicationGroupSecondary`, `WarehouseAdaptive`, `WarehouseInteractive`,
  `PostgresInstance`, `PostgresMirror` (stored procedures:
  `CALL SNOWFLAKE.POSTGRES.CREATE_MIRROR (...)`), `JobService`, the four
  `IcebergTableFrom*` kinds, `UserProgrammaticAccessToken`.
- **Asynchronous objects** (CREATE, then a wait for the status; teardown
  before DROP): `OpenflowDeploymentSnowflakeManaged`,
  `OpenflowDeploymentByoc`, `OpenflowRuntime`, `OpenflowConnector`.
- **Settings on other objects** (ALTER ... SET, read back, ALTER ... UNSET):
  `UserPasswordPolicyAttachment`, `UserSessionPolicyAttachment`,
  `UserAuthenticationPolicyAttachment`, `AccountPasswordPolicyAttachment`,
  `AccountSessionPolicyAttachment`, `AccountAuthenticationPolicyAttachment`,
  `NetworkPolicyAttachment` (one user each; for the account's, see
  below), `AccountParameter`, `ObjectParameter`, `UserPublicKeySet`,
  `TableColumnMaskingPolicyApplication`, `TagAssociation`, `GrantOwnership`.
- **The account's own settings, field by field** (one `ALTER ACCOUNT SET`
  per field that differs, `UNSET` for one the spec drops; parameters read
  from `SHOW PARAMETERS IN ACCOUNT`, policies from `POLICY_REFERENCES`, which
  runs in a warehouse, the resource monitor from `SHOW RESOURCE MONITORS`):
  `CurrentAccount`, `CurrentOrganizationAccount` (whose `name` must be the
  organization account's; it is never renamed). Deleting one unsets only
  what it set (Terraform's delete unsets every parameter and policy).
  `CORTEX_MODELS_ALLOWLIST` is never unset (Snowflake no longer takes the
  default UNSET puts back).
- **Grants and links** (made, found in a list, undone; a changed one is
  undone and made again): `GrantPrivilegesToShare`, `GrantApplicationRole`,
  `TableStorageLifecyclePolicyAttachment`, `TableConstraint`,
  `IntelligenceCortexAgentAttachment`.

An object's name fields (`name`, and `database`, `schema` where it has
them) are identifiers: a plain name is upper-cased as Snowflake does an
unquoted one, any other kept as written, and one written in double quotes
(`'"my name"'` in YAML) keeps its case. An object of the same name made elsewhere is
never taken over: CREATE refuses it, and deleting the Kubernetes object then
drops nothing. A changed name is refused; delete and recreate. A field
Snowflake has no ALTER for (a policy's signature, a file format's type) is
drift that stops, as for the REST kinds that cannot be updated.

Structured values are written element by element: a hybrid table's
`column`s (`{name, type, not_null, default, collate, comment}`), its
constraints and indexes, an external table's columns, a semantic view's
tables, relationships, facts, dimensions and metrics, an external
function's headers. Keywords a value turns on and off (`SECURE`, `NULL` /
`NOT NULL`, `ENFORCED` / `NOT ENFORCED`) follow booleans. A list Snowflake
has no SET for is changed by adding and removing elements: a failover
group's allowed databases, shares and accounts (read back from `SHOW
DATABASES | SHARES IN FAILOVER GROUP`; the group's own account, which SHOW
lists among the allowed, is left out), a replication group's allowed
accounts, an interactive warehouse's tables, a Postgres mirror's tables and
schemas.

**Warehouses.** `MaterializedView` (CREATE runs its query),
`TableStorageLifecyclePolicyAttachment` and `TableConstraint` (read from
`POLICY_REFERENCES` and `INFORMATION_SCHEMA`) and `TagAssociation` (read
with `SYSTEM$GET_TAG`) run in a warehouse: `spec.warehouse`, else the
Connection Secret's key `warehouse`, else the user's default warehouse. The
SQL API request names it; no `USE WAREHOUSE` is sent.

**Secrets Snowflake answers once.** A `UserProgrammaticAccessToken`'s
token is answered only by the statement that makes it: set
`spec.writeSecretRef.name` and it is written to that Secret (key `token`),
which the object owns; it is never in the status. Without it the token is
lost: delete and recreate the object for a new one (Terraform's `keeper`
rotation is not offered). The manager needs `create` and `update` on
Secrets for it.

**Openflow: statements that finish later.** Snowflake answers an Openflow
deployment's, runtime's or connector's statement at once and does the work
after; `SHOW OPENFLOW ...` reports its `status`, read into
`status.atProvider.status`. As Terraform's provider does, the controller
waits after each statement until the status is done (a Snowflake-managed
deployment `ACTIVE`, a BYOC one `INACTIVE` or `ACTIVE`, a runtime `ACTIVE`
or `SUSPENDED`, a connector `RUNNING` or `STOPPED`), sends nothing while one
is in flight (`CREATING`, `PROVISIONING`, `UPDATING`, ...), and reports a
failure status (`CREATE_FAILED`, ...) as an error. It never blocks: it
requeues every 15 seconds with `Ready=False`, reason `Settling`, the status
in the message; after 30 minutes the wait is reported as an error and goes
on with backoff. A change is one `ALTER` per field, each waited for.
Deleting the Kubernetes object runs `TERMINATE` (a connector in a failure
status is sent `STOP` first), waits for `DELETED`, then sends `DROP`: the
finalizer stays until then. A BYOC deployment becomes `ACTIVE` only once
its cloud side is provisioned from the template the Snowflake UI offers;
tear that down before deleting the object. Not offered: renames (as for
every sql kind), `UPGRADE`, `SUSPEND`/`RESUME`, and a connector's
`version_trigger` (`ADD VERSION` with a stop and start around it); a
connector's `runtime` is the runtime's name in the connector's own schema.

**GrantOwnership: what deleting it does.** Ownership cannot be revoked,
only given to another role. Terraform's delete gives it to the role
Terraform runs as (`CURRENT_ROLE()`), which a Kubernetes object has no
stable notion of. Here `spec.revertOwnershipToRole` (required) names the
role ownership goes back to when the object is deleted, and the object's
current grants are copied to it (`COPY CURRENT GRANTS`), so deleting the
Kubernetes object drops no grant. Changing `revertOwnershipToRole` moves
nothing; it is used on delete only.

Only what SHOW, DESCRIBE or a read query answer is compared for drift;
these are written and never compared (each kind's `writeOnly` in
`types.yaml` lists them): secrets and keys (`oauth_client_secret`,
certificates, RSA keys: `UserPublicKeySet` compares their fingerprints),
a policy's `argument` and `return_data_type` and a masking policy's
`exempt_other_policies` (set only when it is made), a resource monitor's
timestamps, a file format's escape and delimiter strings and `null_if`, a
share's `accounts` and `secure_objects_only`, a replica's `as_replica_of`
and `primary`, the OAuth settings inside an API integration's
`API_USER_AUTHENTICATION`, definitions Snowflake re-formats or does not
answer (a view's query, an agent's or MCP server's YAML specification, a
listing's manifest, table columns, constraints and indexes, a semantic
view's definition), and parameters SHOW does not answer (statement
timeouts, retention). A definition that cannot be altered is set only when
the object is made: `status.createdWith` keeps a hash of each such field as
the object was made, and a spec that changes or drops one goes
`Ready=False` (`SyncFailed`) instead of being ignored; change it by
deleting and recreating the object (or put the field back). What
Snowflake makes (a storage integration's IAM user ARN or consent URL, a
SAML integration's metadata, an RSA key's fingerprint) is read into
`status.atProvider`.

Where a kind offers less than Terraform's type (each source's header says
so): `SessionPolicy` and `AuthenticationPolicy` take lists, not Snowflake's
`('ALL')`; `AuthenticationPolicy` has no `mfa_policy`, `pat_policy`,
`client_policy` or `workload_identity_policy` blocks; `ResourceMonitor` has
`suspend_trigger` and `suspend_immediate_trigger`, not `notify_triggers`;
`BackupPolicy` has no `with_retention_lock`; `ExternalAccessIntegration` and
`ApiIntegrationGitRepositoryPrivateLink` take lists, not `all` or `none`;
`GrantPrivilegesToShare` grants on a database, schema, table, view or tag
(not all tables in a schema, nor a function); `NetworkPolicyAttachment` is one
user's, and `AccountParameter` takes a value (string, number, boolean), not
an object's name, so the account's network policy is set in Snowflake;
`ObjectParameter` is for users, warehouses, databases, schemas, tasks,
tables, services and pipes; the account attachments have no
`for_all_person_users` or `for_all_service_users`. `GrantOwnership` gives
one object to an account role (no `database_role_name`, `on.all` or
`on.future`); `TagAssociation` tags one object (Terraform's
`object_identifiers` is a list: one Kubernetes object each) of a type
`SYSTEM$GET_TAG` names as `ALTER` does (not a view or a column);
`TableConstraint` names its table by `database`, `schema` and `table`
(Terraform's `table_id`) and has no `comment`;
`TableStorageLifecyclePolicyAttachment`'s `table_type` is `TABLE` or
`DYNAMIC TABLE`; `UserPublicKeySet` needs `rsa_public_key`, and a key the
spec drops stays set until the object is deleted; `FailoverGroup` and
`ReplicationGroup` take `replication_schedule` as Snowflake writes it
(`10 MINUTE`, `USING CRON ...`), and `FailoverGroup` has no `from_replica`;
`PostgresMirror` takes `refresh_interval` as Snowflake writes it
(`1 minute`) and its name as written (Terraform lower-cases it);
`Listing`, `JobService` take a stage location as one string
(`@db.schema.stage/path`), `Listing` has no later manifest versions, and
`JobService` no `from_specification_template`; `CortexAgent`'s `profile` is
the JSON Snowflake takes; `ExternalTable` takes `file_format` and `location`
as SQL (Terraform passes them as written too); `MaterializedView`,
`ExternalTable`, `HybridTable`, the `IcebergTableFrom*`, warehouses and
`PostgresInstance` have no `tag` and no `or_replace` / `copy_grants`.

### Terraform resources not covered

<!-- BEGIN generated: not covered (krmgen.py, from vendor.yaml hardBlocks and draftNotes) -->
1 of the 154 Terraform gap types have no kind: 1 is not built (`hardBlocks` in `hack/gen/vendor.yaml`).

| Terraform | Status | Why |
| --- | --- | --- |
| `snowflake_execute` | not built | Runs the `execute`, `revert` and `query` statements its configuration holds, as written. A kind for it would let anyone allowed to create the kind run any SQL with the role the manager connects as; no template can check SQL that is free text. Not built: the owner's call whether to offer it at all (for example only behind a manager flag that is off by default). |
<!-- END generated: not covered -->

## For agents (MCP)

`cmd/mcp` serves this repository's kinds over the Model Context Protocol
(stdio), from `types.yaml`, `config/crds` and the live tests' records.
`.mcp.json` registers it, so an agent session opened in this repository
has it without setup. Elsewhere, run it from the repository root:

```sh
go run ./cmd/mcp
```

Its tools: `list_kinds`, `find_kinds_for_terraform`, `describe_kind` (every
field, the REST calls, the live record), `example` (the live-tested spec
where there is one), `validate_manifest` (the CRD schema, as the API server
checks it) and `plan_on_fake` (the controller run against the fake API,
with every REST call it makes). Nothing it does reaches the real API.
krm-rest-runtime's README ("For agents") has the details.

## Regenerate

```sh
hack/gen/generate.sh   # needs python3 + pyyaml, and krm-rest-runtime beside this repo
go test ./...
```
