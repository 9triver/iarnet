#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Experiment Data Analysis and Visualization Script (English Version)
For generating figures for paper experiments section
"""

import os
import re
import pandas as pd
import matplotlib.pyplot as plt
import matplotlib
from matplotlib.lines import Line2D
from matplotlib.colors import rgb2hex, hex2color
import numpy as np
from pathlib import Path

# Set font for English labels
matplotlib.rcParams['font.sans-serif'] = ['DejaVu Sans', 'Arial', 'Liberation Sans', 'Helvetica']
matplotlib.rcParams['axes.unicode_minus'] = False
matplotlib.rcParams['font.family'] = 'sans-serif'

# Set plot style - use a clean, professional style matching paper style
try:
    plt.style.use('seaborn-v0_8-whitegrid')
except:
    try:
        plt.style.use('seaborn-whitegrid')
    except:
        plt.style.use('default')

# Professional color palette (Tableau Classic - harmonious, classic academic colors)
# Tableau Classic palette is widely used in academic papers for its harmonious color combinations
# Colors are well-balanced, professional, and optimized for both color and grayscale printing
# This palette is commonly seen in top-tier journals and provides excellent visual harmony
COLORS = {
    'primary': ['#4E79A7', '#F28E2B', '#59A14F', '#E15759'],  # Blue, Orange, Green, Red (Tableau Classic)
    'secondary': ['#76B7B2', '#EDC948', '#AF7AA1', '#FF9D9A'],  # Teal, Yellow, Purple, Pink
    'accent': ['#4E79A7', '#F28E2B'],  # Blue, Orange
    'task_types': ['#4E79A7', '#F28E2B', '#59A14F'],  # Small (Blue), Medium (Orange), Large (Green)
    'execution': ['#4E79A7', '#F28E2B'],  # Local (Blue), Cross-node (Orange)
    'latency': ['#4E79A7', '#F28E2B', '#59A14F', '#E15759'],  # P50 (Blue), P95 (Orange), P99 (Green), Mean (Red)
}

# Global style parameters - matching paper style
matplotlib.rcParams['figure.figsize'] = (10, 6)
matplotlib.rcParams['font.size'] = 10
matplotlib.rcParams['axes.labelsize'] = 10
matplotlib.rcParams['axes.titlesize'] = 11
matplotlib.rcParams['xtick.labelsize'] = 9
matplotlib.rcParams['ytick.labelsize'] = 9
matplotlib.rcParams['legend.fontsize'] = 9
matplotlib.rcParams['figure.titlesize'] = 12
matplotlib.rcParams['axes.linewidth'] = 1.0
matplotlib.rcParams['grid.linewidth'] = 0.5
matplotlib.rcParams['grid.alpha'] = 0.3
matplotlib.rcParams['grid.color'] = '#E0E0E0'  # Light gray grid
matplotlib.rcParams['axes.spines.top'] = False
matplotlib.rcParams['axes.spines.right'] = False
matplotlib.rcParams['axes.edgecolor'] = '#333333'
matplotlib.rcParams['axes.labelcolor'] = '#333333'
matplotlib.rcParams['text.color'] = '#333333'
matplotlib.rcParams['xtick.color'] = '#333333'
matplotlib.rcParams['ytick.color'] = '#333333'

class ExperimentAnalyzer:
    def __init__(self, result_dir):
        self.result_dir = Path(result_dir)
        self.scenarios = ['1', '2', '4', '8']
        self.data = {}
    
    def get_text_color_for_bar(self, color_hex, alpha=1.0):
        """Determine text color (white or dark) based on bar color brightness"""
        # Convert hex to RGB
        rgb = hex2color(color_hex)
        # Calculate relative luminance (perceived brightness)
        # Using standard formula: 0.299*R + 0.587*G + 0.114*B
        luminance = 0.299 * rgb[0] + 0.587 * rgb[1] + 0.114 * rgb[2]
        # Adjust for alpha (transparency makes color appear lighter)
        effective_luminance = luminance * alpha + (1 - alpha) * 1.0
        # Use white text on dark backgrounds, dark text on light backgrounds
        return 'white' if effective_luminance < 0.6 else '#333333'
        
    def parse_aggregate_metrics(self, scenario):
        """Parse aggregate metrics file"""
        metrics_file = list(self.result_dir.glob(f"{scenario}/aggregate_metrics_*.txt"))[0]
        
        with open(metrics_file, 'r', encoding='utf-8') as f:
            content = f.read()
        
        data = {}
        
        # Parse task statistics
        total_match = re.search(r'总任务数: (\d+)', content)
        success_match = re.search(r'成功任务数: (\d+)', content)
        fail_match = re.search(r'失败任务数: (\d+)', content)
        fail_rate_match = re.search(r'失败率: ([\d.]+)%', content)
        
        data['total_tasks'] = int(total_match.group(1)) if total_match else 0
        data['success_tasks'] = int(success_match.group(1)) if success_match else 0
        data['fail_tasks'] = int(fail_match.group(1)) if fail_match else 0
        data['fail_rate'] = float(fail_rate_match.group(1)) if fail_rate_match else 0
        
        # Parse task type statistics
        task_types = {}
        for task_type in ['Small', 'Medium', 'Large']:
            pattern = rf'{task_type}任务: 总数=(\d+), 成功=(\d+) \(([\d.]+)%\), 失败=(\d+) \(([\d.]+)%\)'
            match = re.search(pattern, content)
            if match:
                task_types[task_type.lower()] = {
                    'total': int(match.group(1)),
                    'success': int(match.group(2)),
                    'success_rate': float(match.group(3)),
                    'fail': int(match.group(4)),
                    'fail_rate': float(match.group(5))
                }
        data['task_types'] = task_types
        
        # Parse execution location statistics
        local_match = re.search(r'本地执行数: (\d+)', content)
        cross_match = re.search(r'跨节点执行数: (\d+)', content)
        local_rate_match = re.search(r'本地执行比例: ([\d.]+)%', content)
        cross_rate_match = re.search(r'跨节点执行比例: ([\d.]+)%', content)
        
        data['local_exec'] = int(local_match.group(1)) if local_match else 0
        data['cross_exec'] = int(cross_match.group(1)) if cross_match else 0
        data['local_rate'] = float(local_rate_match.group(1)) if local_rate_match else 0
        data['cross_rate'] = float(cross_rate_match.group(1)) if cross_rate_match else 0
        
        # Parse latency statistics
        latency_patterns = {
            'avg_resp': r'平均响应时延: ([\d.]+) ms',
            'p50_resp': r'P50 响应时延: ([\d.]+) ms',
            'p95_resp': r'P95 响应时延: ([\d.]+) ms',
            'p99_resp': r'P99 响应时延: ([\d.]+) ms',
            'avg_finish': r'平均完成时延: ([\d.]+) ms',
            'p50_finish': r'P50 完成时延: ([\d.]+) ms',
            'p95_finish': r'P95 完成时延: ([\d.]+) ms',
            'p99_finish': r'P99 完成时延: ([\d.]+) ms',
        }
        
        for key, pattern in latency_patterns.items():
            match = re.search(pattern, content)
            data[key] = float(match.group(1)) if match else 0
        
        # Parse system throughput
        throughput_match = re.search(r'系统吞吐量: ([\d.]+) req/s', content)
        data['throughput'] = float(throughput_match.group(1)) if throughput_match else 0
        
        # Parse resource utilization
        nodes_data = {}
        node_pattern = r'节点: (node\.\d+) \(([^)]+)\)\s+采样次数: (\d+)\s+CPU: 平均 ([\d.]+)%, 最大 ([\d.]+)%\s+内存: 平均 ([\d.]+)%, 最大 ([\d.]+)%\s+GPU: 平均 ([\d.]+)%, 最大 ([\d.]+)%'
        
        for match in re.finditer(node_pattern, content):
            node_name = match.group(1)
            node_id = match.group(2)
            nodes_data[node_name] = {
                'node_id': node_id,
                'cpu_avg': float(match.group(4)),
                'cpu_max': float(match.group(5)),
                'mem_avg': float(match.group(6)),
                'mem_max': float(match.group(7)),
                'gpu_avg': float(match.group(8)),
                'gpu_max': float(match.group(9))
            }
        data['nodes'] = nodes_data
        
        # Parse task distribution by node from aggregate_metrics
        # This section shows which nodes executed which task types
        node_task_distribution = {}
        
        # Parse each task type's distribution
        task_type_map = {'Small': '小', 'Medium': '中', 'Large': '大'}
        next_section_patterns = ['中任务在各节点的执行比例:', '大任务在各节点的执行比例:', '=== 响应时延统计']
        
        for task_type in ['Small', 'Medium', 'Large']:
            # Pattern: node.X (node_id): count 个任务, 占比=percent%
            pattern = rf'node\.(\d+) \(([^)]+)\): (\d+) 个任务, 占比=([\d.]+)%'
            section_pattern = rf'{task_type_map[task_type]}任务在各节点的执行比例:'
            
            # Find the section for this task type
            section_match = re.search(section_pattern, content)
            if section_match:
                # Find all nodes in this section
                start_pos = section_match.end()
                # Find next section
                next_pos = len(content)
                for next_pattern in next_section_patterns:
                    next_match = re.search(next_pattern, content[start_pos:])
                    if next_match:
                        next_pos = min(next_pos, start_pos + next_match.start())
                
                section_content = content[start_pos:next_pos]
                
                for match in re.finditer(pattern, section_content):
                    node_num = int(match.group(1))
                    node_id = match.group(2)
                    task_count = int(match.group(3))
                    task_percent = float(match.group(4))
                    
                    node_name = f'node.{node_num}'
                    if node_name not in node_task_distribution:
                        node_task_distribution[node_name] = {
                            'node_id': node_id,
                            'tasks': {'small': 0, 'medium': 0, 'large': 0},
                            'tasks_percent': {'small': 0, 'medium': 0, 'large': 0}
                        }
                    
                    node_task_distribution[node_name]['tasks'][task_type.lower()] = task_count
                    node_task_distribution[node_name]['tasks_percent'][task_type.lower()] = task_percent
        
        # Ensure all nodes from resource utilization are included (even if they have no tasks)
        for node_name, node_info in nodes_data.items():
            if node_name not in node_task_distribution:
                node_task_distribution[node_name] = {
                    'node_id': node_info['node_id'],
                    'tasks': {'small': 0, 'medium': 0, 'large': 0},
                    'tasks_percent': {'small': 0, 'medium': 0, 'large': 0}
                }
        
        data['node_task_distribution'] = node_task_distribution
        
        return data
    
    def parse_execution_details(self, scenario):
        """Parse detailed execution distribution from CSV file"""
        csv_file = list(self.result_dir.glob(f"{scenario}/experiment_results_*.csv"))[0]
        df = pd.read_csv(csv_file)
        
        # Filter only successful tasks
        df_success = df[df['status'] == 'success'].copy()
        
        details = {
            'by_task_type': {},  # {task_type: {local: count, cross: count}}
            'by_node': {}  # {node_name: {tasks: {}, tasks_percent: {}}}
        }
        
        # Execution distribution by task type
        for task_type in ['small', 'medium', 'large']:
            df_type = df_success[df_success['type'] == task_type]
            local_count = len(df_type[df_type['is_cross_node'] == False])
            cross_count = len(df_type[df_type['is_cross_node'] == True])
            total = local_count + cross_count
            details['by_task_type'][task_type] = {
                'local': local_count,
                'cross': cross_count,
                'local_rate': (local_count / total * 100) if total > 0 else 0,
                'cross_rate': (cross_count / total * 100) if total > 0 else 0
            }
        
        # Use node task distribution from aggregate_metrics (which includes all nodes)
        # This ensures we get all nodes even if they have no tasks
        aggregate_data = self.data[scenario]
        if 'node_task_distribution' in aggregate_data:
            # Use the distribution from aggregate_metrics
            for node_name, node_data in aggregate_data['node_task_distribution'].items():
                details['by_node'][node_name] = {
                    'name': node_name,
                    'tasks': node_data['tasks'].copy(),
                    'tasks_percent': node_data['tasks_percent'].copy()
                }
        else:
            # Fallback to CSV parsing if aggregate_metrics doesn't have the info
            unique_nodes = sorted(df_success['node_id'].unique())
            for idx, node_id in enumerate(unique_nodes):
                df_node = df_success[df_success['node_id'] == node_id]
                node_name = f"node.{idx + 1}"
                details['by_node'][node_name] = {
                    'name': node_name,
                    'tasks': {},
                    'tasks_percent': {}
                }
                
                for task_type in ['small', 'medium', 'large']:
                    count = len(df_node[df_node['type'] == task_type])
                    details['by_node'][node_name]['tasks'][task_type] = count
                    
                    total_task_type = len(df_success[df_success['type'] == task_type])
                    if total_task_type > 0:
                        percent = (count / total_task_type * 100)
                    else:
                        percent = 0
                    details['by_node'][node_name]['tasks_percent'][task_type] = percent
        
        return details
    
    def load_all_data(self):
        """Load data for all scenarios"""
        for scenario in self.scenarios:
            self.data[scenario] = self.parse_aggregate_metrics(scenario)
        # Parse execution details after all aggregate metrics are loaded
        for scenario in self.scenarios:
            self.data[scenario]['execution_details'] = self.parse_execution_details(scenario)
    
    def plot_task_success_rate(self):
        """Figure 1: Task Success Rate Comparison"""
        fig, axes = plt.subplots(1, 2, figsize=(14, 6))
        fig.patch.set_facecolor('white')
        
        scenarios = self.scenarios
        x = np.arange(len(scenarios))
        
        # Left: Overall success rate - use line plot
        success_rates = [100 - self.data[s]['fail_rate'] for s in scenarios]
        
        axes[0].plot(x, success_rates, color=COLORS['primary'][0], marker='o', 
                    markersize=8, linewidth=2.5, alpha=0.9, zorder=3, 
                    markeredgecolor='white', markeredgewidth=1.5)
        axes[0].set_xlabel('Number of Domains', fontweight='normal', fontsize=10)
        axes[0].set_ylabel('Task Success Rate (%)', fontweight='normal', fontsize=10)
        axes[0].set_title('(a) Overall Task Success Rate', fontweight='normal', pad=10, fontsize=11)
        axes[0].set_xticks(x)
        axes[0].set_xticklabels(scenarios, fontsize=9)
        axes[0].tick_params(axis='y', labelsize=9)
        axes[0].set_ylim([0, 105])
        axes[0].grid(True, alpha=0.3, linestyle='-', linewidth=0.5, color='#E0E0E0', zorder=0)
        axes[0].set_axisbelow(True)
        
        # Add value labels with better styling
        for i, v in enumerate(success_rates):
            axes[0].text(i, v + 3, f'{v:.1f}%', ha='center', va='bottom', 
                        fontsize=9, fontweight='normal', color='#333333')
        
        # Right: Success rate by task type - use line plots
        task_types = ['Small', 'Medium', 'Large']
        markers = ['o', 's', '^']
        
        for i, (task_type, marker) in enumerate(zip(task_types, markers)):
            rates = []
            for s in scenarios:
                if task_type.lower() in self.data[s]['task_types']:
                    rates.append(self.data[s]['task_types'][task_type.lower()]['success_rate'])
                else:
                    rates.append(0)
            axes[1].plot(x, rates, label=task_type, color=COLORS['task_types'][i], 
                        marker=marker, markersize=8, linewidth=2.0, alpha=0.9, zorder=3, 
                        markeredgecolor='white', markeredgewidth=1.0)
        
        axes[1].set_xlabel('Number of Domains', fontweight='normal', fontsize=10)
        axes[1].set_ylabel('Success Rate (%)', fontweight='normal', fontsize=10)
        axes[1].set_title('(b) Success Rate by Task Type', fontweight='normal', pad=10, fontsize=11)
        axes[1].set_xticks(x)
        axes[1].set_xticklabels(scenarios, fontsize=9)
        axes[1].tick_params(axis='y', labelsize=9)
        axes[1].legend(loc='lower right', frameon=True, fancybox=False, shadow=False, framealpha=0.9, fontsize=9, edgecolor='#CCCCCC')
        axes[1].set_ylim([0, 105])
        axes[1].grid(True, alpha=0.3, linestyle='-', linewidth=0.5, color='#E0E0E0', zorder=0)
        axes[1].set_axisbelow(True)
        
        plt.tight_layout(pad=3.0)
        plt.savefig(self.result_dir / 'fig1_task_success_rate_en.png', dpi=300, 
                   bbox_inches='tight', facecolor='white', edgecolor='none')
        plt.close()
        print("✓ Generated Figure 1: Task Success Rate Comparison")
    
    def plot_execution_distribution(self):
        """Figure 2: Task Scheduling Distribution"""
        fig, axes = plt.subplots(1, 2, figsize=(14, 6))
        fig.patch.set_facecolor('white')
        
        scenarios = self.scenarios
        x = np.arange(len(scenarios))
        width = 0.6
        
        # Left: Overall scheduling distribution
        ax1 = axes[0]
        local_rates = [self.data[s]['local_rate'] for s in scenarios]
        cross_rates = [self.data[s]['cross_rate'] for s in scenarios]
        
        p1 = ax1.bar(x, local_rates, width, label='Local Execution', 
                    color=COLORS['execution'][0], edgecolor='white', linewidth=2, 
                    alpha=0.9, zorder=3, hatch='')
        p2 = ax1.bar(x, cross_rates, width, bottom=local_rates, 
                    label='Cross-Node Execution', color=COLORS['execution'][1], 
                    edgecolor='white', linewidth=2, alpha=0.9, zorder=3, hatch='///')
        
        ax1.set_xlabel('Number of Domains', fontweight='normal', fontsize=10)
        ax1.set_ylabel('Task Scheduling Ratio (%)', fontweight='normal', fontsize=10)
        ax1.set_title('(a) Overall Task Scheduling Distribution', fontweight='normal', pad=10, fontsize=11)
        ax1.set_xticks(x)
        ax1.set_xticklabels(scenarios, fontsize=9)
        ax1.tick_params(axis='y', labelsize=9)
        ax1.legend(loc='upper left', frameon=True, 
                  fancybox=False, shadow=False, framealpha=0.9, fontsize=9, edgecolor='#CCCCCC')
        ax1.set_ylim([0, 105])
        ax1.grid(True, alpha=0.3, linestyle='-', linewidth=0.5, color='#E0E0E0', zorder=0)
        ax1.set_axisbelow(True)
        
        # Add value labels with adaptive text color
        for i, (l, c) in enumerate(zip(local_rates, cross_rates)):
            if l > 5:
                text_color = self.get_text_color_for_bar(COLORS['execution'][0], alpha=0.9)
                ax1.text(i, l/2, f'{l:.1f}%', ha='center', va='center', 
                        fontsize=8, color=text_color, fontweight='normal')
            if c > 5:
                text_color = self.get_text_color_for_bar(COLORS['execution'][1], alpha=0.9)
                ax1.text(i, l + c/2, f'{c:.1f}%', ha='center', va='center', 
                        fontsize=8, color=text_color, fontweight='normal')
        
        # Right: Scheduling distribution by task type
        ax2 = axes[1]
        task_types = ['small', 'medium', 'large']
        task_labels = ['Small', 'Medium', 'Large']
        x_pos = np.arange(len(scenarios))
        width_bar = 0.25
        
        # Store bar objects for value labels
        local_bars_list = []
        cross_bars_list = []
        
        for i, (task_type, task_label) in enumerate(zip(task_types, task_labels)):
            local_vals = []
            cross_vals = []
            for s in scenarios:
                details = self.data[s]['execution_details']['by_task_type']
                if task_type in details:
                    local_vals.append(details[task_type]['local_rate'])
                    cross_vals.append(details[task_type]['cross_rate'])
                else:
                    local_vals.append(0)
                    cross_vals.append(0)
            
            # Plot local execution (increased alpha for deeper colors)
            # Different hatch patterns for each task type: Small='', Medium='|||', Large='---'
            hatch_patterns_local = ['', '|||', '---']
            local_bars = ax2.bar(x_pos + i*width_bar, local_vals, width_bar, 
                   label=f'{task_label} (Local)', 
                   color=COLORS['task_types'][i], edgecolor='white', 
                   linewidth=1.5, alpha=0.9, zorder=3, hatch=hatch_patterns_local[i])
            local_bars_list.append((local_bars, local_vals, COLORS['task_types'][i]))
            
            # Plot cross-node execution (increased alpha for deeper colors)
            # Different hatch patterns for cross-node: Small='///', Medium='\\\\\\', Large='xxx'
            hatch_patterns_cross = ['///', '\\\\\\', 'xxx']
            cross_bars = ax2.bar(x_pos + i*width_bar, cross_vals, width_bar, 
                   bottom=local_vals, label=f'{task_label} (Cross)', 
                   color=COLORS['task_types'][i], edgecolor='white', 
                   linewidth=1.5, alpha=0.6, zorder=3, hatch=hatch_patterns_cross[i])
            cross_bars_list.append((cross_bars, cross_vals, local_vals, COLORS['task_types'][i]))
        
        # Add value labels for local execution with adaptive text color
        for bars, vals, color in local_bars_list:
            for bar, val in zip(bars, vals):
                if val > 2:  # Only label if value is significant
                    text_color = self.get_text_color_for_bar(color, alpha=0.9)
                    ax2.text(bar.get_x() + bar.get_width()/2, bar.get_height()/2, 
                           f'{val:.1f}%', ha='center', va='center', 
                           fontsize=7, fontweight='normal', color=text_color)
        
        # Add value labels for cross-node execution with adaptive text color
        for bars, cross_vals, local_vals, color in cross_bars_list:
            for bar, cross_val, local_val in zip(bars, cross_vals, local_vals):
                if cross_val > 2:  # Only label if value is significant
                    bottom = local_val
                    height = cross_val
                    text_color = self.get_text_color_for_bar(color, alpha=0.6)
                    ax2.text(bar.get_x() + bar.get_width()/2, bottom + height/2, 
                           f'{cross_val:.1f}%', ha='center', va='center', 
                           fontsize=7, fontweight='normal', color=text_color)
        
        ax2.set_xlabel('Number of Domains', fontweight='normal', fontsize=10)
        ax2.set_ylabel('Task Scheduling Ratio (%)', fontweight='normal', fontsize=10)
        ax2.set_title('(b) Task Scheduling Distribution by Task Type', fontweight='normal', pad=10, fontsize=11)
        ax2.set_xticks(x_pos + width_bar)
        ax2.set_xticklabels(scenarios, fontsize=9)
        ax2.tick_params(axis='y', labelsize=9)
        ax2.legend(loc='upper left', frameon=True, 
                  fancybox=False, shadow=False, framealpha=0.9, fontsize=9, edgecolor='#CCCCCC')
        ax2.set_ylim([0, 105])
        ax2.grid(True, alpha=0.3, linestyle='-', linewidth=0.5, color='#E0E0E0', zorder=0)
        ax2.set_axisbelow(True)
        
        plt.tight_layout(pad=3.0)
        plt.savefig(self.result_dir / 'fig2_task_scheduling_distribution_en.png', dpi=300, 
                   bbox_inches='tight', facecolor='white', edgecolor='none')
        plt.close()
        print("✓ Generated Figure 2: Task Scheduling Distribution")
    
    def plot_node_task_distribution(self):
        """Figure 3: Task Distribution by Node"""
        fig = plt.figure(figsize=(18, 5))
        fig.patch.set_facecolor('white')
        
        scenarios = self.scenarios
        
        # Subplot 1, 2, 3: Node-level distribution for scenarios with 2, 4, 8 nodes
        plot_scenarios = ['2', '4', '8']
        
        for idx, scenario in enumerate(plot_scenarios):
            ax = plt.subplot(1, 3, idx + 1)
            details = self.data[scenario]['execution_details']['by_node']
            
            # Sort nodes by node number (node.1, node.2, ..., node.8)
            def get_node_number(node_name):
                match = re.search(r'node\.(\d+)', node_name)
                return int(match.group(1)) if match else 999
            
            nodes = sorted(details.keys(), key=get_node_number)
            # Convert node.X to domain.X for display
            node_names = [details[node]['name'].replace('node.', 'domain.') for node in nodes]
            task_types = ['small', 'medium', 'large']
            task_labels = ['Small', 'Medium', 'Large']
            
            x_node = np.arange(len(nodes))
            width_node = 0.25
            
            # Create grouped bars for each task type - showing percentages
            # Different hatch patterns for each task type: Small='', Medium='|||', Large='---'
            hatch_patterns = ['', '|||', '---']
            bars_list = []
            for i, (task_type, task_label) in enumerate(zip(task_types, task_labels)):
                percents = [details[node]['tasks_percent'].get(task_type, 0) for node in nodes]
                counts = [details[node]['tasks'].get(task_type, 0) for node in nodes]
                bars = ax.bar(x_node + i*width_node, percents, width_node, 
                             label=task_label, color=COLORS['task_types'][i], 
                             edgecolor='white', linewidth=1.5, alpha=0.9, zorder=3,
                             hatch=hatch_patterns[i])
                bars_list.append((bars, counts))
            
            ax.set_xlabel('Domain', fontweight='normal', fontsize=9)
            ax.set_ylabel('Task Percentage (%)', fontweight='normal', fontsize=9)
            ax.set_title(f'({chr(97+idx)}) {scenario}-Domain Scenario: Task Distribution by Domain', 
                        fontweight='normal', pad=8, fontsize=10)
            ax.set_xticks(x_node + width_node)
            ax.set_xticklabels(node_names, rotation=45, ha='right', fontsize=8)
            ax.tick_params(axis='y', labelsize=8)
            
            # Add value labels on top of bars (show percentage instead of count)
            for bars, counts in bars_list:
                for bar, count in zip(bars, counts):
                    height = bar.get_height()
                    if height > 0.5:  # Label if height is significant
                        ax.text(bar.get_x() + bar.get_width()/2, height + 0.5, 
                               f'{height:.1f}%', ha='center', va='bottom', 
                               fontsize=7, fontweight='normal', color='#333333')
            
            # Place legend in upper right corner of first subplot
            if idx == 0:  # Only show legend on first subplot
                ax.legend(loc='upper right', 
                         frameon=True, fancybox=False, shadow=False, framealpha=0.9, fontsize=8, edgecolor='#CCCCCC')
            max_percent = max([max([details[node]['tasks_percent'].get(t, 0) 
                                    for node in nodes]) 
                              for t in task_types])
            ax.set_ylim([0, max(max_percent * 1.15, 10)])  # At least 10% for visibility
            ax.grid(True, alpha=0.3, linestyle='-', linewidth=0.5, color='#E0E0E0', zorder=0)
            ax.set_axisbelow(True)
        
        plt.tight_layout(pad=3.0)  # Adjust layout
        plt.subplots_adjust(wspace=0.15)  # Reduce horizontal spacing between subplots
        plt.savefig(self.result_dir / 'fig3_node_task_distribution_en.png', dpi=300, 
                   bbox_inches='tight', facecolor='white', edgecolor='none')
        plt.close()
        print("✓ Generated Figure 3: Task Distribution by Node")
    
    def plot_latency_and_throughput(self):
        """Figure 4: Response Latency and Throughput Comparison (Side by Side)"""
        fig, axes = plt.subplots(1, 2, figsize=(14, 6))
        fig.patch.set_facecolor('white')
        
        scenarios = self.scenarios
        x = np.arange(len(scenarios))
        
        # Left subplot: Response latency metrics
        ax1 = axes[0]
        metrics = ['p50_resp', 'p95_resp', 'p99_resp', 'avg_resp']
        labels = ['P50', 'P95', 'P99', 'Mean']
        markers = ['o', 's', '^', 'D']  # Different markers for each line
        linewidths = [2.5, 2.5, 2.5, 2.5]
        
        for i, (metric, label, marker) in enumerate(zip(metrics, labels, markers)):
            values = [self.data[s][metric] for s in scenarios]
            ax1.plot(x, values, label=label, color=COLORS['latency'][i], 
                    marker=marker, markersize=6, linewidth=2.0, 
                    alpha=0.9, zorder=3, markeredgecolor='white', markeredgewidth=1.0)
            
            # Add value labels on data points
            for j, v in enumerate(values):
                # For log scale, position label slightly above the point
                # Adjust offset based on value to avoid overlapping
                offset = v * 0.15  # 15% above the point
                ax1.text(j, v * 1.15, f'{v:.1f}', ha='center', va='bottom', 
                        fontsize=7, fontweight='normal', color=COLORS['latency'][i])
        
        ax1.set_xlabel('Number of Domains', fontweight='normal', fontsize=10)
        ax1.set_ylabel('Response Latency (ms)', fontweight='normal', fontsize=10)
        ax1.set_title('(a) Response Latency Statistics', fontweight='normal', pad=10, fontsize=11)
        ax1.set_xticks(x)
        ax1.set_xticklabels(scenarios, fontsize=9)
        ax1.tick_params(axis='y', labelsize=9)
        ax1.legend(loc='upper right', frameon=True, fancybox=False, shadow=False, framealpha=0.9, fontsize=9, edgecolor='#CCCCCC')
        ax1.set_yscale('log')
        ax1.grid(True, alpha=0.3, linestyle='-', linewidth=0.5, color='#E0E0E0', zorder=0)
        ax1.set_axisbelow(True)
        
        # Right subplot: System throughput
        ax2 = axes[1]
        throughputs = [self.data[s]['throughput'] for s in scenarios]
        
        # Use line plot with markers
        ax2.plot(x, throughputs, color=COLORS['primary'][0], marker='o', 
                markersize=8, linewidth=2.5, alpha=0.9, zorder=3, 
                markeredgecolor='white', markeredgewidth=1.5)
        
        ax2.set_xlabel('Number of Domains', fontweight='normal', fontsize=10)
        ax2.set_ylabel('Throughput (req/s)', fontweight='normal', fontsize=10)
        ax2.set_title('(b) System Throughput', fontweight='normal', pad=10, fontsize=11)
        ax2.set_xticks(x)
        ax2.set_xticklabels(scenarios, fontsize=9)
        ax2.tick_params(axis='y', labelsize=9)
        ax2.grid(True, alpha=0.3, linestyle='-', linewidth=0.5, color='#E0E0E0', zorder=0)
        ax2.set_axisbelow(True)
        
        # Add value labels with better styling
        for i, v in enumerate(throughputs):
            ax2.text(i, v + 0.5, f'{v:.2f}', ha='center', va='bottom', 
                    fontsize=9, fontweight='normal', color='#333333')
        
        plt.tight_layout(pad=3.0)
        plt.savefig(self.result_dir / 'fig4_latency_throughput_en.png', dpi=300, 
                   bbox_inches='tight', facecolor='white', edgecolor='none')
        plt.close()
        print("✓ Generated Figure 4: Response Latency and Throughput Comparison")
    
    def plot_local_node_resource_utilization(self):
        """Figure 5: Local Node Resource Utilization"""
        fig, ax = plt.subplots(figsize=(10, 6))
        fig.patch.set_facecolor('white')
        
        scenarios = self.scenarios
        # Resource colors using Tableau Classic (harmonious, classic academic colors)
        resource_colors = ['#4E79A7', '#F28E2B', '#59A14F']  # CPU (Blue), Memory (Orange), GPU (Green)
        
        x = np.arange(len(scenarios))
        
        # Find local node (typically node.1)
        local_node_name = None
        for s in scenarios:
            nodes = self.data[s]['nodes']
            if nodes:
                if 'node.1' in nodes:
                    local_node_name = 'node.1'
                    break
                elif nodes:
                    local_node_name = list(nodes.keys())[0]
                    break
        
        if local_node_name:
            # Prepare data for grouped bar chart
            cpu_avg = []
            mem_avg = []
            gpu_avg = []
            
            for s in scenarios:
                nodes = self.data[s]['nodes']
                if local_node_name in nodes:
                    node_data = nodes[local_node_name]
                    cpu_avg.append(node_data['cpu_avg'])
                    mem_avg.append(node_data['mem_avg'])
                    gpu_avg.append(node_data['gpu_avg'])
                else:
                    cpu_avg.append(0)
                    mem_avg.append(0)
                    gpu_avg.append(0)
            
            # Plot grouped bar chart
            group_width = 0.8
            bar_width = group_width / 3
            
            for i, (ca, ma, ga) in enumerate(zip(cpu_avg, mem_avg, gpu_avg)):
                x_pos = x[i]
                x_cpu = x_pos - group_width/3
                x_mem = x_pos
                x_gpu = x_pos + group_width/3
                
                # CPU bar - vertical lines
                ax.bar(x_cpu, ca, bar_width, label='CPU' if i == 0 else '', 
                       color=resource_colors[0], alpha=0.9, edgecolor='white', linewidth=1.5,
                       hatch='|||')
                
                # Memory bar - horizontal lines
                ax.bar(x_mem, ma, bar_width, label='Memory' if i == 0 else '', 
                       color=resource_colors[1], alpha=0.9, edgecolor='white', linewidth=1.5,
                       hatch='---')
                
                # GPU bar - diagonal lines
                ax.bar(x_gpu, ga, bar_width, label='GPU' if i == 0 else '', 
                       color=resource_colors[2], alpha=0.9, edgecolor='white', linewidth=1.5,
                       hatch='///')
                
                # Add value labels
                if ca > 0:
                    ax.text(x_cpu, ca + 1, f'{ca:.1f}', ha='center', va='bottom', fontsize=9, fontweight='normal')
                if ma > 0:
                    ax.text(x_mem, ma + 1, f'{ma:.1f}', ha='center', va='bottom', fontsize=9, fontweight='normal')
                if ga > 0:
                    ax.text(x_gpu, ga + 1, f'{ga:.1f}', ha='center', va='bottom', fontsize=9, fontweight='normal')
        
        ax.set_xlabel('Number of Domains', fontweight='normal', fontsize=10)
        ax.set_ylabel('Resource Utilization (%)', fontweight='normal', fontsize=10)
        ax.set_title('Local Node Resource Utilization', fontweight='normal', pad=10, fontsize=11)
        ax.set_xticks(x)
        ax.set_xticklabels(scenarios, fontsize=9)
        ax.tick_params(axis='y', labelsize=9)
        legend_elements = [
            plt.Rectangle((0,0),1,1, facecolor=resource_colors[0], alpha=0.9, edgecolor='white', label='CPU', hatch='|||'),
            plt.Rectangle((0,0),1,1, facecolor=resource_colors[1], alpha=0.9, edgecolor='white', label='Memory', hatch='---'),
            plt.Rectangle((0,0),1,1, facecolor=resource_colors[2], alpha=0.9, edgecolor='white', label='GPU', hatch='///')
        ]
        ax.legend(handles=legend_elements, loc='upper right', frameon=True, fancybox=False, shadow=False, framealpha=0.9, fontsize=9, edgecolor='#CCCCCC')
        ax.set_ylim([0, 105])
        ax.grid(True, alpha=0.3, linestyle='-', linewidth=0.5, color='#E0E0E0', zorder=0)
        ax.set_axisbelow(True)
        
        plt.tight_layout(pad=3.0)
        plt.savefig(self.result_dir / 'fig5_local_node_resource_utilization_en.png', dpi=300, 
                   bbox_inches='tight', facecolor='white', edgecolor='none')
        plt.close()
        print("✓ Generated Figure 5: Local Node Resource Utilization")
    
    def plot_resource_utilization(self):
        """Figure 6: Resource Utilization - All scenarios in one figure"""
        # Create a figure with 4 subplots: one for each scenario
        fig = plt.figure(figsize=(16, 10))
        fig.patch.set_facecolor('white')
        
        scenarios = self.scenarios
        # Resource colors using Tableau Classic (harmonious, classic academic colors)
        resource_colors = ['#4E79A7', '#F28E2B', '#59A14F']  # CPU (Blue), Memory (Orange), GPU (Green)
        
        # Create grid layout: 2 rows, 2 columns (4 subplots)
        gs = fig.add_gridspec(2, 2, hspace=0.3, wspace=0.25)
        
        # Subplots: Each scenario's resource utilization
        subplot_positions = [
            (0, 0),  # Top left: 1 node
            (0, 1),  # Top right: 2 nodes
            (1, 0),  # Bottom left: 4 nodes
            (1, 1),  # Bottom right: 8 nodes
        ]
        
        for idx, scenario in enumerate(scenarios):
            nodes = self.data[scenario]['nodes']
            if not nodes:
                continue
            
            row, col = subplot_positions[idx]
            ax = fig.add_subplot(gs[row, col])
            
            # Get all nodes for this scenario and sort them
            all_nodes = sorted(nodes.keys())
            
            # Prepare data
            cpu_avg = []
            cpu_max = []
            mem_avg = []
            mem_max = []
            gpu_avg = []
            gpu_max = []
            
            for node_name in all_nodes:
                node_data = nodes[node_name]
                cpu_avg.append(node_data['cpu_avg'])
                cpu_max.append(node_data['cpu_max'])
                mem_avg.append(node_data['mem_avg'])
                mem_max.append(node_data['mem_max'])
                gpu_avg.append(node_data['gpu_avg'])
                gpu_max.append(node_data['gpu_max'])
            
            # Plot grouped bar chart
            x_nodes = np.arange(len(all_nodes))
            group_width = 0.8
            bar_width = group_width / 3
            
            for i, (ca, cm, ma, mm, ga, gm) in enumerate(zip(cpu_avg, cpu_max, mem_avg, mem_max, gpu_avg, gpu_max)):
                x_pos = x_nodes[i]
                x_cpu = x_pos - group_width/3
                x_mem = x_pos
                x_gpu = x_pos + group_width/3
                
                # CPU bar - vertical lines
                ax.bar(x_cpu, ca, bar_width, label='CPU' if i == 0 else '', 
                       color=resource_colors[0], alpha=0.9, edgecolor='white', linewidth=1.5,
                       hatch='|||')
                
                # Memory bar - horizontal lines
                ax.bar(x_mem, ma, bar_width, label='Memory' if i == 0 else '', 
                       color=resource_colors[1], alpha=0.9, edgecolor='white', linewidth=1.5,
                       hatch='---')
                
                # GPU bar - diagonal lines
                ax.bar(x_gpu, ga, bar_width, label='GPU' if i == 0 else '', 
                       color=resource_colors[2], alpha=0.9, edgecolor='white', linewidth=1.5,
                       hatch='///')
                
                # Add value labels (smaller font for subplots)
                if ca > 5:
                    ax.text(x_cpu, ca + 1, f'{ca:.1f}', ha='center', va='bottom', fontsize=6, fontweight='normal')
                if ma > 5:
                    ax.text(x_mem, ma + 1, f'{ma:.1f}', ha='center', va='bottom', fontsize=6, fontweight='normal')
                if ga > 5:
                    ax.text(x_gpu, ga + 1, f'{ga:.1f}', ha='center', va='bottom', fontsize=6, fontweight='normal')
            
            ax.set_xlabel('Node', fontweight='normal', fontsize=9)
            ax.set_ylabel('Resource Utilization (%)', fontweight='normal', fontsize=9)
            ax.set_title(f'({chr(97+idx)}) {scenario}-Domain Scenario', fontweight='normal', pad=8, fontsize=10)
            ax.set_xticks(x_nodes)
            ax.set_xticklabels(all_nodes, rotation=45, ha='right', fontsize=8)
            ax.tick_params(axis='y', labelsize=8)
            if idx == 0:  # Only show legend on first subplot
                legend_elements = [
                    plt.Rectangle((0,0),1,1, facecolor=resource_colors[0], alpha=0.9, edgecolor='white', label='CPU', hatch='|||'),
                    plt.Rectangle((0,0),1,1, facecolor=resource_colors[1], alpha=0.9, edgecolor='white', label='Memory', hatch='---'),
                    plt.Rectangle((0,0),1,1, facecolor=resource_colors[2], alpha=0.9, edgecolor='white', label='GPU', hatch='///')
                ]
                ax.legend(handles=legend_elements, loc='upper right', frameon=True, fancybox=False, shadow=False, framealpha=0.9, fontsize=7, edgecolor='#CCCCCC')
            ax.set_ylim([0, 105])
            ax.grid(True, alpha=0.3, linestyle='-', linewidth=0.5, color='#E0E0E0', zorder=0)
            ax.set_axisbelow(True)
        
        plt.savefig(self.result_dir / 'fig6_resource_utilization_en.png', dpi=300, 
                   bbox_inches='tight', facecolor='white', edgecolor='none')
        plt.close()
        print("✓ Generated Figure 6: Resource Utilization (All Scenarios)")
    
    def plot_resource_utilization_trends(self):
        """Figure 7: Resource Utilization Trends - Domain.1 vs Other Domains"""
        fig, axes = plt.subplots(1, 2, figsize=(14, 6))
        fig.patch.set_facecolor('white')
        
        scenarios = self.scenarios
        x = np.arange(len(scenarios))
        # Resource colors using Tableau Classic
        resource_colors = ['#4E79A7', '#F28E2B', '#59A14F']  # CPU (Blue), Memory (Orange), GPU (Green)
        resource_labels = ['CPU', 'Memory', 'GPU']
        markers = ['o', 's', '^']  # Different markers for each resource
        
        # Left subplot: Domain.1 resource utilization across scenarios
        ax1 = axes[0]
        
        # Find domain.1 data
        cpu_domain1 = []
        mem_domain1 = []
        gpu_domain1 = []
        
        for s in scenarios:
            nodes = self.data[s]['nodes']
            if 'node.1' in nodes:
                node_data = nodes['node.1']
                cpu_domain1.append(node_data['cpu_avg'])
                mem_domain1.append(node_data['mem_avg'])
                gpu_domain1.append(node_data['gpu_avg'])
            else:
                cpu_domain1.append(0)
                mem_domain1.append(0)
                gpu_domain1.append(0)
        
        # Plot lines for domain.1
        ax1.plot(x, cpu_domain1, label='CPU', color=resource_colors[0], marker=markers[0],
                markersize=8, linewidth=2.5, alpha=0.9, zorder=3, markeredgecolor='white', markeredgewidth=1.5)
        ax1.plot(x, mem_domain1, label='Memory', color=resource_colors[1], marker=markers[1],
                markersize=8, linewidth=2.5, alpha=0.9, zorder=3, markeredgecolor='white', markeredgewidth=1.5)
        ax1.plot(x, gpu_domain1, label='GPU', color=resource_colors[2], marker=markers[2],
                markersize=8, linewidth=2.5, alpha=0.9, zorder=3, markeredgecolor='white', markeredgewidth=1.5)
        
        ax1.set_xlabel('Number of Domains', fontweight='normal', fontsize=10)
        ax1.set_ylabel('Resource Utilization (%)', fontweight='normal', fontsize=10)
        ax1.set_title('(a) Domain.1 Resource Utilization', fontweight='normal', pad=10, fontsize=11)
        ax1.set_xticks(x)
        ax1.set_xticklabels(scenarios, fontsize=9)
        ax1.tick_params(axis='y', labelsize=9)
        ax1.legend(loc='best', frameon=True, fancybox=False, shadow=False, framealpha=0.9, fontsize=9, edgecolor='#CCCCCC')
        ax1.set_ylim([0, 105])
        ax1.grid(True, alpha=0.3, linestyle='-', linewidth=0.5, color='#E0E0E0', zorder=0)
        ax1.set_axisbelow(True)
        
        # Right subplot: Average resource utilization of other domains
        ax2 = axes[1]
        
        cpu_other_avg = []
        mem_other_avg = []
        gpu_other_avg = []
        
        for s in scenarios:
            nodes = self.data[s]['nodes']
            if not nodes:
                cpu_other_avg.append(0)
                mem_other_avg.append(0)
                gpu_other_avg.append(0)
                continue
            
            # Get all nodes except node.1
            other_nodes = [node_name for node_name in nodes.keys() if node_name != 'node.1']
            
            if not other_nodes:
                # If no other nodes, use 0
                cpu_other_avg.append(0)
                mem_other_avg.append(0)
                gpu_other_avg.append(0)
            else:
                # Calculate average of other domains
                cpu_values = [nodes[node_name]['cpu_avg'] for node_name in other_nodes]
                mem_values = [nodes[node_name]['mem_avg'] for node_name in other_nodes]
                gpu_values = [nodes[node_name]['gpu_avg'] for node_name in other_nodes]
                
                cpu_other_avg.append(np.mean(cpu_values))
                mem_other_avg.append(np.mean(mem_values))
                gpu_other_avg.append(np.mean(gpu_values))
        
        # Plot lines for other domains average
        ax2.plot(x, cpu_other_avg, label='CPU', color=resource_colors[0], marker=markers[0],
                markersize=8, linewidth=2.5, alpha=0.9, zorder=3, markeredgecolor='white', markeredgewidth=1.5)
        ax2.plot(x, mem_other_avg, label='Memory', color=resource_colors[1], marker=markers[1],
                markersize=8, linewidth=2.5, alpha=0.9, zorder=3, markeredgecolor='white', markeredgewidth=1.5)
        ax2.plot(x, gpu_other_avg, label='GPU', color=resource_colors[2], marker=markers[2],
                markersize=8, linewidth=2.5, alpha=0.9, zorder=3, markeredgecolor='white', markeredgewidth=1.5)
        
        ax2.set_xlabel('Number of Domains', fontweight='normal', fontsize=10)
        ax2.set_ylabel('Resource Utilization (%)', fontweight='normal', fontsize=10)
        ax2.set_title('(b) Average Resource Utilization of Other Domains', fontweight='normal', pad=10, fontsize=11)
        ax2.set_xticks(x)
        ax2.set_xticklabels(scenarios, fontsize=9)
        ax2.tick_params(axis='y', labelsize=9)
        ax2.legend(loc='best', frameon=True, fancybox=False, shadow=False, framealpha=0.9, fontsize=9, edgecolor='#CCCCCC')
        ax2.set_ylim([0, 105])
        ax2.grid(True, alpha=0.3, linestyle='-', linewidth=0.5, color='#E0E0E0', zorder=0)
        ax2.set_axisbelow(True)
        
        plt.tight_layout(pad=3.0)
        plt.savefig(self.result_dir / 'fig7_resource_utilization_trends_en.png', dpi=300, 
                   bbox_inches='tight', facecolor='white', edgecolor='none')
        plt.close()
        print("✓ Generated Figure 7: Resource Utilization Trends")
    
    def plot_comprehensive_comparison(self):
        """Figure 8: Comprehensive Performance Comparison (Normalized)"""
        fig, ax = plt.subplots(figsize=(12, 7))
        fig.patch.set_facecolor('white')
        
        scenarios = self.scenarios
        
        # Normalize metrics (relative to maximum)
        metrics_data = {
            'Success Rate': [100 - self.data[s]['fail_rate'] for s in scenarios],
            'Throughput': [self.data[s]['throughput'] for s in scenarios],
        }
        
        # Normalize (relative to maximum)
        normalized_data = {}
        for metric, values in metrics_data.items():
            max_val = max(values)
            normalized_data[metric] = [v/max_val*100 for v in values]
        
        x = np.arange(len(scenarios))
        width = 0.38
        
        p1 = ax.bar(x - width/2, normalized_data['Success Rate'], width, 
                   label='Success Rate (Normalized)', color=COLORS['execution'][0], 
                   edgecolor='white', linewidth=2, alpha=0.9, zorder=3, hatch='')
        p2 = ax.bar(x + width/2, normalized_data['Throughput'], width, 
                   label='Throughput (Normalized)', color=COLORS['execution'][1], 
                   edgecolor='white', linewidth=2, alpha=0.9, zorder=3, hatch='///')
        
        ax.set_xlabel('Number of Domains', fontweight='medium')
        ax.set_ylabel('Normalized Performance (%)', fontweight='medium')
        ax.set_title('Comprehensive Performance Comparison (Normalized)', 
                    fontweight='bold', pad=15)
        ax.set_xticks(x)
        ax.set_xticklabels(scenarios)
        ax.legend(loc='upper left', frameon=True, fancybox=True, shadow=True, framealpha=0.9)
        ax.set_ylim([0, 110])
        ax.grid(True, alpha=0.3, linestyle='-', linewidth=0.5, color='#E0E0E0', zorder=0)
        ax.set_axisbelow(True)
        
        # Add value labels
        for i, (sr, tp) in enumerate(zip(normalized_data['Success Rate'], normalized_data['Throughput'])):
            ax.text(i - width/2, sr + 1, f'{sr:.1f}%', ha='center', va='bottom', 
                   fontsize=10, fontweight='bold', color='#2C3E50')
            ax.text(i + width/2, tp + 1, f'{tp:.1f}%', ha='center', va='bottom', 
                   fontsize=10, fontweight='bold', color='#2C3E50')
        
        plt.tight_layout()
        plt.savefig(self.result_dir / 'fig8_comprehensive_comparison_en.png', dpi=300, 
                   bbox_inches='tight', facecolor='white', edgecolor='none')
        plt.close()
        print("✓ Generated Figure 8: Comprehensive Performance Comparison")
    
    def generate_summary_table(self):
        """Generate summary table (CSV format)"""
        rows = []
        for scenario in self.scenarios:
            data = self.data[scenario]
            row = {
                'Scenario': f'{scenario} Nodes',
                'Total Tasks': data['total_tasks'],
                'Success Rate (%)': round(100 - data['fail_rate'], 2),
                'Local Exec Rate (%)': round(data['local_rate'], 2),
                'Cross-Node Exec Rate (%)': round(data['cross_rate'], 2),
                'Avg Response Latency (ms)': round(data['avg_resp'], 2),
                'P95 Response Latency (ms)': round(data['p95_resp'], 2),
                'Throughput (req/s)': round(data['throughput'], 2),
                'Avg CPU Utilization (%)': round(np.mean([n['cpu_avg'] for n in data['nodes'].values()]) if data['nodes'] else 0, 2),
                'Avg Memory Utilization (%)': round(np.mean([n['mem_avg'] for n in data['nodes'].values()]) if data['nodes'] else 0, 2),
                'Avg GPU Utilization (%)': round(np.mean([n['gpu_avg'] for n in data['nodes'].values()]) if data['nodes'] else 0, 2),
            }
            rows.append(row)
        
        df = pd.DataFrame(rows)
        df.to_csv(self.result_dir / 'experiment_summary_en.csv', index=False, encoding='utf-8-sig')
        print("✓ Generated summary table: experiment_summary_en.csv")
        print("\nSummary table preview:")
        print(df.to_string(index=False))
    
    def run_all(self):
        """Run all analysis and plotting"""
        print("Starting experiment data analysis...")
        self.load_all_data()
        print(f"Loaded data for {len(self.data)} scenarios\n")
        
        print("Generating figures...")
        self.plot_task_success_rate()
        self.plot_execution_distribution()
        self.plot_node_task_distribution()
        self.plot_latency_and_throughput()
        self.plot_local_node_resource_utilization()
        self.plot_resource_utilization()
        self.plot_resource_utilization_trends()
        
        print("\nGenerating summary table...")
        self.generate_summary_table()
        
        print("\n" + "="*50)
        print("All figures generated successfully!")
        print(f"Output directory: {self.result_dir}")
        print("="*50)

if __name__ == '__main__':
    result_dir = '/home/zhangyx/iarnet/experiment/result'
    analyzer = ExperimentAnalyzer(result_dir)
    analyzer.run_all()

