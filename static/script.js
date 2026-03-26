// 全局变量
let scanResults = [];

document.addEventListener('DOMContentLoaded', function() {
    const scanButton = document.getElementById('scanButton');
    const startIPInput = document.getElementById('startIP');
    const endIPInput = document.getElementById('endIP');
    const statsCard = document.getElementById('statsCard');
    const resultsCard = document.getElementById('resultsCard');
    const ipGrid = document.getElementById('ipGrid');
    const onlineCount = document.getElementById('onlineCount');
    const arpCount = document.getElementById('arpCount');
    const icmpCount = document.getElementById('icmpCount');
    const loadingOverlay = document.getElementById('loadingOverlay');

    // 扫描按钮点击事件
    scanButton.addEventListener('click', startScan);

    // 输入框回车事件
    [startIPInput, endIPInput].forEach(input => {
        input.addEventListener('keypress', function(e) {
            if (e.key === 'Enter') {
                startScan();
            }
        });
    });

    async function startScan() {
        const startIP = startIPInput.value.trim();
        const endIP = endIPInput.value.trim();

        // 验证输入
        if (!startIP || !endIP) {
            showNotification('请输入起始IP和结束IP', 'error');
            return;
        }

        if (!isValidIP(startIP) || !isValidIP(endIP)) {
            showNotification('请输入有效的IP地址', 'error');
            return;
        }

        // 显示加载状态
        scanButton.disabled = true;
        scanButton.innerHTML = '<i class="fas fa-spinner fa-spin"></i><span>扫描中...</span>';
        loadingOverlay.style.display = 'flex';

        // 隐藏之前的结果
        statsCard.style.display = 'none';
        resultsCard.style.display = 'none';

        try {
            // 发送扫描请求
            const response = await fetch('/api/scan', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    startIP: startIP,
                    endIP: endIP
                })
            });

            const data = await response.json();

            if (data.success) {
                // 保存结果
                scanResults = data.results;
                // 显示结果
                displayResults(data.results);
                showNotification('扫描完成！', 'success');
            } else {
                showNotification('扫描失败: ' + data.message, 'error');
            }
        } catch (error) {
            console.error('扫描错误:', error);
            showNotification('扫描过程中发生错误: ' + error.message, 'error');
        } finally {
            // 恢复按钮状态
            scanButton.disabled = false;
            scanButton.innerHTML = '<i class="fas fa-play"></i><span>开始扫描</span>';
            loadingOverlay.style.display = 'none';
        }
    }

    function displayResults(results) {
        // 清空网格
        ipGrid.innerHTML = '';

        // 计算统计数据
        const onlineResults = results.filter(r => r.status === 'online');
        const arpResults = results.filter(r => r.method === 'arp');
        const icmpResults = results.filter(r => r.method === 'icmp');

        // 更新统计信息
        onlineCount.textContent = onlineResults.length;
        arpCount.textContent = arpResults.length;
        icmpCount.textContent = icmpResults.length;

        // 显示统计卡片
        statsCard.style.display = 'block';

        // 创建网格单元格
        results.forEach((result, index) => {
            const cell = document.createElement('div');
            cell.className = 'ip-cell';
            
            // 添加IP地址数据属性（用于提示框）
            cell.setAttribute('data-ip', result.ip);
            cell.setAttribute('data-status', result.status);
            cell.setAttribute('data-method', result.method || 'none');
            cell.setAttribute('data-mac', result.mac || '未获取');
            cell.setAttribute('data-latency', result.latency || 0);
            cell.setAttribute('data-timestamp', result.timestamp || '');

            // 根据状态设置样式
            if (result.status === 'online') {
                cell.classList.add('online');
                // 根据检测方法添加不同的标记
                if (result.method === 'arp') {
                    cell.classList.add('arp');
                } else if (result.method === 'icmp') {
                    cell.classList.add('icmp');
                }
            } else {
                cell.classList.add('offline');
            }

            // 显示编号（1-255）
            cell.textContent = index + 1;

            // 添加点击事件
            cell.addEventListener('click', () => {
                showIPDetails(result);
            });

            // 添加动画延迟
            cell.style.animation = `fadeIn 0.3s ease-out ${index * 0.005}s`;

            ipGrid.appendChild(cell);
        });

        // 显示结果卡片
        resultsCard.style.display = 'block';
    }

    // IP地址验证
    function isValidIP(ip) {
        const ipPattern = /^(\d{1,3}\.){3}\d{1,3}$/;
        if (!ipPattern.test(ip)) return false;
        
        const parts = ip.split('.');
        return parts.every(part => {
            const num = parseInt(part, 10);
            return num >= 0 && num <= 255;
        });
    }

    // 显示通知
    function showNotification(message, type = 'info') {
        const notification = document.createElement('div');
        notification.className = `notification notification-${type}`;
        notification.style.cssText = `
            position: fixed;
            top: 20px;
            right: 20px;
            padding: 16px 24px;
            background: white;
            border-radius: 12px;
            box-shadow: 0 8px 24px rgba(0, 0, 0, 0.15);
            z-index: 10000;
            animation: slideIn 0.3s ease-out;
            display: flex;
            align-items: center;
            gap: 12px;
            min-width: 300px;
            max-width: 400px;
        `;

        const icons = {
            success: '<i class="fas fa-check-circle" style="color: #10b981; font-size: 1.5rem;"></i>',
            error: '<i class="fas fa-times-circle" style="color: #ef4444; font-size: 1.5rem;"></i>',
            info: '<i class="fas fa-info-circle" style="color: #3b82f6; font-size: 1.5rem;"></i>'
        };

        notification.innerHTML = `
            ${icons[type]}
            <span style="flex: 1; font-size: 0.95rem; color: #1f2937;">${message}</span>
            <button onclick="this.parentElement.remove()" style="background: none; border: none; cursor: pointer; color: #6b7280; padding: 4px;">
                <i class="fas fa-times"></i>
            </button>
        `;

        document.body.appendChild(notification);

        // 自动关闭
        setTimeout(() => {
            notification.style.animation = 'slideOut 0.3s ease-out';
            setTimeout(() => notification.remove(), 300);
        }, 3000);
    }

    // 导出结果
    window.exportResults = function() {
        if (scanResults.length === 0) {
            showNotification('没有可导出的结果', 'error');
            return;
        }

        // 生成CSV内容
        const csvContent = generateCSV(scanResults);
        
        // 创建下载链接
        const blob = new Blob(['\ufeff' + csvContent], { type: 'text/csv;charset=utf-8;' });
        const link = document.createElement('a');
        const url = URL.createObjectURL(blob);
        
        link.setAttribute('href', url);
        link.setAttribute('download', `scan_results_${new Date().toISOString().slice(0, 10)}.csv`);
        link.style.visibility = 'hidden';
        
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        
        showNotification('导出成功！', 'success');
    };

    // 生成CSV
    function generateCSV(results) {
        let csv = 'IP地址,状态,检测方法,MAC地址,响应延迟,扫描时间\n';
        
        results.forEach(result => {
            const status = result.status === 'online' ? '在线' : '离线';
            const method = result.method === 'arp' ? 'ARP' : result.method === 'icmp' ? 'ICMP Ping' : '未检测';
            const mac = result.mac || '未获取';
            const latency = result.latency ? result.latency + 'ms' : '0ms';
            const timestamp = result.timestamp || '';
            
            csv += `"${result.ip}","${status}","${method}","${mac}","${latency}","${timestamp}"\n`;
        });
        
        return csv;
    }

    // 清除结果
    window.clearResults = function() {
        scanResults = [];
        statsCard.style.display = 'none';
        resultsCard.style.display = 'none';
        ipGrid.innerHTML = '';
        showNotification('已清除扫描结果', 'info');
    };

    // 显示IP详情
    function showIPDetails(result) {
        const modal = document.getElementById('ipDetailModal');
        const modalBody = document.getElementById('modalBody');
        
        const status = result.status === 'online' ? '在线' : '离线';
        const statusColor = result.status === 'online' ? '#10b981' : '#9ca3af';
        const statusIcon = result.status === 'online' ? 'fa-wifi' : 'fa-wifi';

        // 检测方法显示
        let methodText = '未检测';
        let methodColor = '#9ca3af';
        let methodIcon = 'fa-question';
        if (result.method === 'arp') {
            methodText = 'ARP';
            methodColor = '#f59e0b';
            methodIcon = 'fa-exchange-alt';
        } else if (result.method === 'icmp') {
            methodText = 'ICMP Ping';
            methodColor = '#3b82f6';
            methodIcon = 'fa-signal';
        }

        // 构建详情内容
        let detailsHTML = `
            <div class="detail-row">
                <div class="detail-label">
                    <i class="fas fa-server"></i> IP 地址
                </div>
                <div class="detail-value">
                    <strong>${result.ip}</strong>
                </div>
            </div>
            
            <div class="detail-row">
                <div class="detail-label">
                    <i class="fas ${statusIcon}" style="color: ${statusColor}"></i> 状态
                </div>
                <div class="detail-value">
                    <span class="status-badge" style="background: ${statusColor}20; color: ${statusColor};">
                        ${status}
                    </span>
                </div>
            </div>
        `;

        if (result.status === 'online') {
            detailsHTML += `
                <div class="detail-row">
                    <div class="detail-label">
                        <i class="fas ${methodIcon}" style="color: ${methodColor}"></i> 检测方法
                    </div>
                    <div class="detail-value">
                        <span class="method-badge" style="background: ${methodColor}20; color: ${methodColor};">
                            ${methodText}
                        </span>
                    </div>
                </div>
            `;

            if (result.mac && result.mac !== '') {
                detailsHTML += `
                    <div class="detail-row">
                        <div class="detail-label">
                            <i class="fas fa-microchip"></i> MAC 地址
                        </div>
                        <div class="detail-value">
                            <code style="background: #f3f4f6; padding: 6px 12px; border-radius: 6px; font-family: 'Courier New', monospace;">${result.mac}</code>
                        </div>
                    </div>
                `;
            }

            if (result.latency !== undefined && result.latency !== null && result.latency > 0) {
                const latencyColor = result.latency < 50 ? '#10b981' : result.latency < 100 ? '#f59e0b' : '#ef4444';
                detailsHTML += `
                    <div class="detail-row">
                        <div class="detail-label">
                            <i class="fas fa-stopwatch" style="color: ${latencyColor}"></i> 响应延迟
                        </div>
                        <div class="detail-value">
                            <strong style="color: ${latencyColor}">${result.latency} ms</strong>
                        </div>
                    </div>
                `;
            }
        }

        if (result.timestamp) {
            detailsHTML += `
                <div class="detail-row">
                    <div class="detail-label">
                        <i class="fas fa-clock"></i> 扫描时间
                    </div>
                    <div class="detail-value">
                        ${result.timestamp}
                    </div>
                </div>
            `;
        }

        modalBody.innerHTML = detailsHTML;
        modal.style.display = 'block';
    }

    // 关闭模态框
    window.closeModal = function() {
        const modal = document.getElementById('ipDetailModal');
        modal.style.display = 'none';
    };

    // 点击模态框外部关闭
    window.addEventListener('click', function(e) {
        const modal = document.getElementById('ipDetailModal');
        if (e.target === modal) {
            closeModal();
        }
    });

    // ESC键关闭模态框
    document.addEventListener('keydown', function(e) {
        if (e.key === 'Escape') {
            closeModal();
        }
    });

    // 添加CSS动画
    const style = document.createElement('style');
    style.textContent = `
        @keyframes fadeIn {
            from {
                opacity: 0;
                transform: translateY(10px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }

        @keyframes slideIn {
            from {
                transform: translateX(400px);
                opacity: 0;
            }
            to {
                transform: translateX(0);
                opacity: 1;
            }
        }

        @keyframes slideOut {
            from {
                transform: translateX(0);
                opacity: 1;
            }
            to {
                transform: translateX(400px);
                opacity: 0;
            }
        }

        .detail-row {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 16px 0;
            border-bottom: 1px solid #e5e7eb;
        }

        .detail-row:last-child {
            border-bottom: none;
        }

        .detail-label {
            font-weight: 500;
            color: #6b7280;
            font-size: 0.95rem;
            display: flex;
            align-items: center;
            gap: 8px;
        }

        .detail-value {
            color: #1f2937;
            font-size: 0.95rem;
        }

        .status-badge, .method-badge {
            padding: 6px 16px;
            border-radius: 20px;
            font-weight: 600;
            font-size: 0.85rem;
        }

        code {
            font-size: 0.85rem;
            color: #4b5563;
        }
    `;
    document.head.appendChild(style);
});