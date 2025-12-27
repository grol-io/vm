#include <inttypes.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#ifndef DEBUG
#define DEBUG 0
#endif

#if DEBUG
#define DEBUG_PRINT(fmt, ...)                                                  \
  do {                                                                         \
    fprintf(stderr, fmt, __VA_ARGS__);                                         \
  } while (0)
#define DEBUG_ASSERT(expr)                                                     \
  do {                                                                         \
    if (!(expr)) {                                                             \
      fprintf(stderr, "Assertion failed: %s, file %s, line %d\n", #expr,       \
              __FILE__, __LINE__);                                             \
      exit(1);                                                                 \
    }                                                                          \
  } while (0)
#else
#define DEBUG_PRINT(fmt, ...)                                                  \
  do {                                                                         \
  } while (0)
#define DEBUG_ASSERT(expr)                                                     \
  do {                                                                         \
  } while (0)
#endif

typedef int64_t Operation;

uint8_t get_opcode(Operation op) { return (uint8_t)(op & 0xFF); }

int64_t get_operand(Operation op) { return (int64_t)(op >> 8); }

typedef struct CPU {
  int64_t accumulator;
  int64_t pc;
  Operation *program;
  size_t program_size;
} CPU;

void run_program(CPU *cpu) {
  int64_t end = (int64_t)(cpu->program_size);
  while (cpu->pc < end) {
    Operation op = cpu->program[cpu->pc];
    uint8_t opcode = get_opcode(op);
    int64_t operand = get_operand(op);
    switch (opcode) {
    case 1: // LoadI
      DEBUG_PRINT("LoadI %" PRId64 " at PC %" PRId64 "\n", operand, cpu->pc);
      cpu->accumulator = operand;
      break;
    case 2: // AddI
      DEBUG_PRINT("AddI %" PRId64 " at PC %" PRId64 "\n", operand, cpu->pc);
      cpu->accumulator += operand;
      break;
    case 3: // JNZ
      DEBUG_PRINT("JNZ %" PRId64 " at PC %" PRId64 "\n", operand, cpu->pc);
      if (cpu->accumulator != 0) {
        cpu->pc += operand;
        continue;
      }
      break;
    case 4: // Load
      DEBUG_PRINT("Load   at PC %" PRId64 ", offset: %" PRId64 "\n", cpu->pc,
                  operand);
      DEBUG_ASSERT(cpu->pc + operand >= 0 &&
                   (size_t)(cpu->pc + operand) < cpu->program_size);
      cpu->accumulator = (int64_t)cpu->program[cpu->pc + operand];
      DEBUG_PRINT("       loaded value: %" PRId64 "\n", cpu->accumulator);
      break;
    case 5: // Add
      DEBUG_PRINT("Add    at PC %" PRId64 ", offset: %" PRId64 "\n", cpu->pc,
                  operand);
      DEBUG_ASSERT(cpu->pc + operand >= 0 &&
                   (size_t)(cpu->pc + operand) < cpu->program_size);
      cpu->accumulator += (int64_t)cpu->program[cpu->pc + operand];
      DEBUG_PRINT("       result: %" PRId64 "\n", cpu->accumulator);
      break;
    case 6: // Store
      DEBUG_PRINT("Store  at PC %" PRId64 ", offset: %" PRId64
                  ", value: %" PRId64 "\n",
                  cpu->pc, operand, cpu->accumulator);
      DEBUG_ASSERT(cpu->pc + operand >= 0 &&
                   (size_t)(cpu->pc + operand) < cpu->program_size);
      cpu->program[cpu->pc + operand] = (Operation)cpu->accumulator;
      break;
    case 7: // Sys
    {
      uint8_t syscallid = operand & 0xFF;
      int64_t syscallarg = operand >> 8;
      switch (syscallid) {
      case 1: // Exit
        printf("Exit Syscall (%d) at PC %" PRId64 ": %" PRId64 "\n", syscallid,
               cpu->pc, syscallarg);
        // note that switching to int return and using return syscallarg; adds
        // 1s to linux/amd64 times (2.6s->3.5s) [but not on apple silicon]
        exit(syscallarg);
      case 2: // Sleep
        if (syscallarg < 0 || syscallarg > 1000) {
          fprintf(stderr,
                  "ERR: Sleep syscall argument out of range at PC %" PRId64
                  ": %" PRId64 "\n",
                  cpu->pc, syscallarg);
          exit(1);
        }
        printf("Sleeping for %" PRId64 " milliseconds at PC %" PRId64 "\n",
               syscallarg, cpu->pc);
        usleep(syscallarg * 1000);
        break;
      default:
        fprintf(stderr, "ERR: Unknown syscall %d at PC %" PRId64 "\n",
                syscallid, cpu->pc);
        exit(1);
      }
    } break;
    default:
      fprintf(stderr, "ERR: Unknown opcode %d at PC %" PRId64 "\n", opcode,
              cpu->pc);
      exit(1);
    }
    cpu->pc++;
  }
  printf("Program finished. Accumulator: %" PRId64 "\n", cpu->accumulator);
}

#define HEADER "\x01GROL VM" // matches cpu.HEADER
#define INSTR_SIZE sizeof(Operation)

int main(int argc, char **argv) {
  if (argc < 2) {
    fprintf(stderr, "Usage: %s <program.vm>\n", argv[0]);
    return 1;
  }
  const char *filename = argv[1];
  FILE *f = fopen(filename, "rb");
  if (!f) {
    perror("Failed to open file");
    return 1;
  }
  CPU cpu = {0};
  fseek(f, 0, SEEK_END);
  cpu.program_size = (ftell(f) - (sizeof(HEADER) - 1)) /
                     INSTR_SIZE; // packed size of Operation in file - header.
  cpu.program = malloc(cpu.program_size * INSTR_SIZE);
  if (!cpu.program) {
    perror("Failed to allocate memory for program");
    fclose(f);
    return 1;
  }
  fseek(f, 0, SEEK_SET);
  char header[sizeof(HEADER)];
  header[sizeof(HEADER) - 1] = '\0';
  if (fread(header, sizeof(HEADER) - 1, 1, f) != 1) {
    perror("Failed to read header");
    fclose(f);
    free(cpu.program);
    return 1;
  }
  if (strncmp(header, HEADER, sizeof(HEADER) - 1) != 0) {
    fprintf(stderr, "Invalid header: %s\n", header);
    fclose(f);
    free(cpu.program);
    return 1;
  }
  if (fread(cpu.program, INSTR_SIZE, cpu.program_size, f) != cpu.program_size) {
    perror("Failed to read operation");
    fclose(f);
    free(cpu.program);
    return 1;
  }
  fclose(f);
  printf("Loaded program with %zu operations\n", cpu.program_size);
  run_program(&cpu);
  free(cpu.program);
  return 0;
}
