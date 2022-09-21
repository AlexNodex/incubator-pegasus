// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

#pragma once

#include <dsn/dist/replication/replication_app_base.h>
#include <dsn/tool-api/zlocks.h>
#include <dsn/utility/filesystem.h>

#include "replica/replica.h"
#include "replica/replica_stub.h"

namespace dsn {
namespace replication {

class replica;

///
/// Replica backup process
///
///  ----------->  Invalid  ----------------|
///  |                |                     |
///  |                v       Error/Cancel  |
///  |            Checkpoint -------------->|
///  |                |                     |
///  |                v       Error/Cancel  |
///  |           Checkpoined -------------->|
///  |                |                     |
///  |                v       Error/Cancel  |
///  |            Uploading  -------------->|
///  |                |                     |
///  |                v                     |
///  |             Succeed                  |
///  |                |                     |
///  |                v                     |
///  |<--  Async-clear backup files  <------|

class replica_backup_manager : replica_base
{
public:
    explicit replica_backup_manager(replica *r);
    ~replica_backup_manager();

    void on_backup(const backup_request &request, /*out*/ backup_response &response);

private:
    void try_to_checkpoint(const int64_t &backup_id, /*out*/ backup_response &response);
    void try_to_upload(const std::string &provider_type,
                       const std::string &root_path,
                       const std::string &app_name,
                       /*out*/ backup_response &response);
    void start_checkpointing(int64_t backup_id, /*out*/ backup_response &response);
    void report_checkpointing(/*out*/ backup_response &response);
    void start_uploading(const std::string &provider_name,
                         const std::string &root_path,
                         const std::string &app_name,
                         /*out*/ backup_response &response);
    void report_uploading(/*out*/ backup_response &response);
    void upload_completed(/*out*/ backup_response &response);
    void fill_response_unlock(/*out*/ backup_response &response);

    void generate_checkpoint();
    bool set_backup_metadata_unlock(const std::string &local_checkpoint_dir,
                                    int64_t checkpoint_decree,
                                    int64_t checkpoint_timestamp);
    void upload_checkpoint(const std::string &provider_name,
                           const std::string &root_path,
                           const std::string &app_name);
    void upload_file(dist::block_service::block_filesystem *fs,
                     const std::string &remote_partition_dir,
                     const file_meta &f_meta,
                     int32_t next_index);
    void upload_file_completed(dist::block_service::block_filesystem *fs,
                               const std::string &remote_partition_dir);
    error_code write_file_to_blockfs(dist::block_service::block_filesystem *fs,
                                     const std::string &remote_dir,
                                     const std::string &file_name,
                                     const blob &buffer);
    int32_t calc_upload_progress();

    void clear_context();
    void background_clear_backup_checkpoint(int64_t backup_id);
    void clear_backup_checkpoint(int64_t backup_id);
    bool cleanup_backup_task(task_ptr task_);

    task_tracker *tracker() { return _replica->tracker(); }

    // local backup directory: <backup_dir>/<backup_id>
    std::string get_local_checkpoint_dir()
    {
        zauto_read_lock l(_lock);
        return utils::filesystem::path_combine(_replica->_app->backup_dir(),
                                               std::to_string(_backup_id));
    }

    backup_status::type get_backup_status()
    {
        zauto_read_lock l(_lock);
        return _status;
    }

    void set_checkpoint_err(const error_code &ec)
    {
        zauto_write_lock l(_lock);
        _checkpoint_err = ec;
    }

    void set_upload_err(const error_code &ec)
    {
        zauto_write_lock l(_lock);
        _upload_err = ec;
    }

private:
    replica *_replica;
    replica_stub *_stub;

    friend class replica;
    friend class replica_stub;
    friend class replica_backup_manager_test;

    zrwlock_nr _lock; // {
    backup_status::type _status{backup_status::UNINITIALIZED};
    int64_t _backup_id{0};
    error_code _checkpoint_err{ERR_OK};
    error_code _upload_err{ERR_OK};
    cold_backup_metadata _backup_metadata;
    task_ptr _checkpointing_task;
    task_ptr _uploading_task;
    // file_name -> upload task
    std::map<std::string, task_ptr> _upload_files_task;
    std::atomic<uint64_t> _upload_file_size{0};
    // }
};

} // namespace replication
} // namespace dsn
