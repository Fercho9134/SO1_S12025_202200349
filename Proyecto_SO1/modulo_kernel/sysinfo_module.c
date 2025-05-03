#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/string.h>
#include <linux/init.h>
#include <linux/proc_fs.h>
#include <linux/seq_file.h>
#include <linux/mm.h>
#include <linux/sched.h>
#include <linux/timer.h>
#include <linux/jiffies.h>
#include <linux/uaccess.h>
#include <linux/tty.h>
#include <linux/sched/signal.h>
#include <linux/fs.h>
#include <linux/slab.h>
#include <linux/sched/mm.h>
#include <linux/binfmts.h>
#include <linux/timekeeping.h>
#include <linux/cgroup.h>
#include <linux/delay.h>

#define CONTAINER_ID_FLAG "-id "
#define ID_LIMIT 100
#define BUFFER_CAPACITY 256
#define PROC_FS_NAME "sysinfo_202200349"
#define CMDLINE_LIMIT 256

void deallocate(char *pointer);
static char *retrieve_command(struct task_struct *process);
static char *isolate_container_id(const char *command);
static ssize_t fetch_file_content(const char *file_path, char *buffer, size_t buffer_size);
static char *compute_memory_consumption(const char *container_id, unsigned long total_memory);
static char *compute_cpu_consumption(const char *container_id);
static char *compute_disk_consumption(const char *container_id);
static char *compute_io_activity(const char *container_id);
static int render_metrics(struct seq_file *seq, void *v);
static int open_proc_file(struct inode *inode, struct file *file);
static int __init init(void);
static void __exit clean(void);

void deallocate(char *pointer) {
    if (pointer) kfree(pointer);
}

static char *retrieve_command(struct task_struct *process) {
    struct mm_struct *memory_map;
    char *command_buffer, *temp;
    unsigned long args_start, args_end, env_start;
    int index, length;

    command_buffer = kmalloc(CMDLINE_LIMIT, GFP_KERNEL);
    if (!command_buffer) return NULL;

    memory_map = get_task_mm(process);
    if (!memory_map) {
        kfree(command_buffer);
        return NULL;
    }

    down_read(&memory_map->mmap_lock);
    args_start = memory_map->arg_start;
    args_end = memory_map->arg_end;
    env_start = memory_map->env_start;
    up_read(&memory_map->mmap_lock);

    length = args_end - args_start;
    if (length > CMDLINE_LIMIT - 1) length = CMDLINE_LIMIT - 1;

    if (access_process_vm(process, args_start, command_buffer, length, 0) != length) {
        mmput(memory_map);
        kfree(command_buffer);
        return NULL;
    }
    command_buffer[length] = '\0';

    temp = command_buffer;
    for (index = 0; index < length; index++)
        if (temp[index] == '\0') temp[index] = ' ';

    mmput(memory_map);
    return command_buffer;
}

static char *isolate_container_id(const char *command) {
    static char container_id[ID_LIMIT];
    char *marker_position = strstr(command, CONTAINER_ID_FLAG);
    if (marker_position) {
        marker_position += strlen(CONTAINER_ID_FLAG);
        sscanf(marker_position, "%99s", container_id);
        return container_id;
    }
    return NULL;
}

static ssize_t fetch_file_content(const char *file_path, char *buffer, size_t buffer_size) {
    struct file *target_file;
    ssize_t bytes_read = 0;
    loff_t offset = 0;

    target_file = filp_open(file_path, O_RDONLY, 0);
    if (IS_ERR(target_file)) return -ENOENT;

    bytes_read = kernel_read(target_file, buffer, buffer_size - 1, &offset);
    if (bytes_read < 0) {
        filp_close(target_file, NULL);
        return bytes_read;
    }

    buffer[bytes_read] = '\0';
    filp_close(target_file, NULL);
    return bytes_read;
}

static char *compute_memory_consumption(const char *container_id, unsigned long total_memory) {
    char path[BUFFER_CAPACITY], data[BUFFER_CAPACITY];
    unsigned long long memory_used = 0;
    unsigned long long usage_percentage = 0;
    char *result;

    snprintf(path, BUFFER_CAPACITY, "/sys/fs/cgroup/system.slice/docker-%s.scope/memory.current", container_id);

    if (fetch_file_content(path, data, BUFFER_CAPACITY) > 0) sscanf(data, "%llu", &memory_used);

    memory_used /= (1024 * 1024);
    total_memory /= 1024;

    if (total_memory > 0) usage_percentage = (memory_used * 10000) / total_memory;

    result = kmalloc(32, GFP_KERNEL);
    if (!result) return NULL;

    snprintf(result, 32, "%llu.%02llu", usage_percentage / 100, usage_percentage % 100);
    return result;
}

static char *compute_cpu_consumption(const char *container_id) {
    char path[BUFFER_CAPACITY], data[BUFFER_CAPACITY];
    unsigned long long cpu_usage_start = 0, cpu_usage_end = 0;
    char *result;

    snprintf(path, BUFFER_CAPACITY, "/sys/fs/cgroup/system.slice/docker-%s.scope/cpu.stat", container_id);

    if (fetch_file_content(path, data, BUFFER_CAPACITY) <= 0) return NULL;
    sscanf(data, "usage_usec %llu", &cpu_usage_start);

    msleep(700);

    if (fetch_file_content(path, data, BUFFER_CAPACITY) <= 0) return NULL;
    sscanf(data, "usage_usec %llu", &cpu_usage_end);

    unsigned long long delta = cpu_usage_end - cpu_usage_start;
    unsigned long long cpu_usage = (delta) / 100;

    result = kmalloc(32, GFP_KERNEL);
    if (!result) return NULL;

    snprintf(result, 32, "%llu.%02llu", cpu_usage / 100, cpu_usage % 100);
    return result;
}

static char *compute_disk_consumption(const char *container_id) {
    char path[BUFFER_CAPACITY], data[BUFFER_CAPACITY];
    unsigned long long read_bytes = 0, written_bytes = 0, total_bytes = 0;
    char *result;

    snprintf(path, BUFFER_CAPACITY, "/sys/fs/cgroup/system.slice/docker-%s.scope/io.stat", container_id);

    if (fetch_file_content(path, data, BUFFER_CAPACITY) > 0) {
        char *read_pos = strstr(data, "rbytes=");
        if (read_pos) sscanf(read_pos + strlen("rbytes="), "%llu", &read_bytes);

        char *write_pos = strstr(data, "wbytes=");
        if (write_pos) sscanf(write_pos + strlen("wbytes="), "%llu", &written_bytes);
    }

    read_bytes /= (1024 * 1024);
    written_bytes /= (1024 * 1024);
    total_bytes = read_bytes + written_bytes;

    result = kmalloc(64, GFP_KERNEL);
    if (!result) return NULL;

    snprintf(result, 64, "%llu", total_bytes);
    return result;
}

static char *compute_io_activity(const char *container_id) {
    char path[BUFFER_CAPACITY], data[BUFFER_CAPACITY];
    unsigned long long read_ops = 0, write_ops = 0, total_ops = 0;
    char *result;

    snprintf(path, BUFFER_CAPACITY, "/sys/fs/cgroup/system.slice/docker-%s.scope/io.stat", container_id);

    if (fetch_file_content(path, data, BUFFER_CAPACITY) > 0) {
        char *read_pos = strstr(data, "rios=");
        if (read_pos) sscanf(read_pos + strlen("rios="), "%llu", &read_ops);

        char *write_pos = strstr(data, "wios=");
        if (write_pos) sscanf(write_pos + strlen("wios="), "%llu", &write_ops);
    }

    total_ops = read_ops + write_ops;

    result = kmalloc(64, GFP_KERNEL);
    if (!result) return NULL;

    snprintf(result, 64, "%llu", total_ops);
    return result;
}

static int render_metrics(struct seq_file *seq, void *v) {
    struct sysinfo system_info;
    struct task_struct *process;
    int is_first_process = 1;
    si_meminfo(&system_info);

    unsigned long total_memory = system_info.totalram * system_info.mem_unit / (1024 * 1024);
    unsigned long free_memory = system_info.freeram * system_info.mem_unit / (1024 * 1024);
    unsigned long used_memory = total_memory - free_memory;

    struct file *file;
    char buffer[256];
    loff_t offset = 0;
    file = filp_open("/proc/stat", O_RDONLY, 0);
    if (!IS_ERR(file)) {
        kernel_read(file, buffer, sizeof(buffer) - 1, &offset);
        filp_close(file, NULL);
    }

    unsigned long user, nice, system, idle, iowait, irq, softirq, steal;
    sscanf(buffer, "cpu %lu %lu %lu %lu %lu %lu %lu %lu", &user, &nice, &system, &idle, &iowait, &irq, &softirq, &steal);
    unsigned long total_time = user + nice + system + idle + iowait + irq + softirq + steal;
    unsigned long busy_time = total_time - idle;
    unsigned long cpu_usage = (busy_time * 100) / total_time;

    seq_printf(seq, "  {\n");
    seq_printf(seq, "\"SystemMetrics\": \n");
    seq_printf(seq, "  {\n");
    seq_printf(seq, "    \"TotalMemory\": %lu,\n", total_memory);
    seq_printf(seq, "    \"FreeMemory\": %lu,\n", free_memory);
    seq_printf(seq, "    \"UsedMemory\": %lu,\n", used_memory);
    seq_printf(seq, "    \"CpuUsage\": %lu\n", cpu_usage);
    seq_printf(seq, "  },\n");
    seq_printf(seq, "\"ProcessDetails\": [\n");

    for_each_process(process) {
        if (strcmp(process->comm, "containerd-shim") == 0) {
            char *command = retrieve_command(process);

            if (!is_first_process) seq_printf(seq, ",\n");
            else is_first_process = 0;

            char *container_id = isolate_container_id(command);
            char *disk_usage = compute_disk_consumption(container_id);
            char *cpu_usage = compute_cpu_consumption(container_id);
            char *memory_usage = compute_memory_consumption(container_id, total_memory);
            char *io_activity = compute_io_activity(container_id);

            seq_printf(seq, "  {\n");
            seq_printf(seq, "    \"pid\": %d,\n", process->pid);
            seq_printf(seq, "    \"name\": \"%s\",\n", process->comm);
            seq_printf(seq, "    \"ContainerId\": \"%s\",\n", container_id ? container_id : "N/A");
            seq_printf(seq, "    \"MemoryUsage\": %s,\n", memory_usage);
            seq_printf(seq, "    \"CpuUsage\": %s,\n", cpu_usage);
            seq_printf(seq, "    \"DiskUsage\": %s,\n", disk_usage);
            seq_printf(seq, "    \"IoActivity\": %s\n", io_activity);
            seq_printf(seq, "  }");

            deallocate(disk_usage);
            deallocate(memory_usage);
            deallocate(cpu_usage);
            deallocate(io_activity);
            if (command) kfree(command);
        }
    }

    seq_printf(seq, "\n]\n}\n");
    return 0;
}

static int open_proc_file(struct inode *inode, struct file *file) {
    return single_open(file, render_metrics, NULL);
}

static const struct proc_ops proc_operations = {
    .proc_open = open_proc_file,
    .proc_read = seq_read,
};

static int __init init(void) {
    proc_create(PROC_FS_NAME, 0, NULL, &proc_operations);
    printk(KERN_INFO "Module initialized\n");
    return 0;
}

static void __exit clean(void) {
    remove_proc_entry(PROC_FS_NAME, NULL);
    printk(KERN_INFO "Module cleaned up\n");
}

MODULE_LICENSE("GPL");
MODULE_AUTHOR("Fernando Alvarado");
MODULE_DESCRIPTION("Module to monitor Docker container resource usage");
MODULE_VERSION("1.0");

module_init(init);
module_exit(clean);