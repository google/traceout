// Copyright 2014 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*
Package ftrace reads the Linux kernel binary trace pipe files
/sys/kernel/debug/tracing/per_cpu/cpu<n>/trace_pipe_raw and uses
the event format files to produce (nearly) identical output to
/sys/kernel/debug/tracing/trace while allowing offloading a
significant portion of the work to a remote processor.

Basics:
Create an ftrace.FileProvider that can read and write the various
tracing and proc files needed.  NewLocalFileProvider() can be used
to create one that reads the files from the local path, or
NewRecordingFileProvider() and NewTestFileProvider() can be used to
create one that records and replays accesses for testing.
For tracing a remote device, implement FileProvider over your
choice of IPC.

Create an ftrace object with NewFtrace, create the events
with ftrace.NewEventType(), call ftrace.PrepareCapture()
to open the trace pipes, enable the event types with etype.Enable()
and the tracing with ftrace.Enable(), and then read the events
from ftrace.Capture().
*/

package ftrace
