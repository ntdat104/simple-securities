// vn30_cagr.js
// Chạy bằng lệnh: node vn30_cagr.js

// Danh sách mã cổ phiếu VN30 tiêu biểu
const vn30Symbols = [
    "FPT", "VCB", "HPG", "VIC", "VHM", "VNM", "TCB", "CTG", "BID", "MBB",
    "VPB", "MWG", "MSN", "ACB", "STB", "GAS", "SAB", "SSI", "HDB", "VIB",
    "TPB", "POW", "BVH", "REE", "PNJ", "VRE", "KDH", "GVR", "SHB", "LPB"
];

async function fetchStockData(symbol) {
    // Lấy mốc thời gian 10 năm trước và hiện tại (Unix timestamp bằng giây)
    const now = Math.floor(Date.now() / 1000);
    const tenYearsAgo = now - (10 * 365 * 24 * 60 * 60);

    const url = `https://api.dnse.com.vn/chart-api/v2/ohlcs/stock?from=${tenYearsAgo}&to=${now}&symbol=${symbol}&resolution=1D`;

    try {
        const response = await fetch(url);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        return data; // { t: [...], c: [...] }
    } catch (error) {
        console.error(`Lỗi khi tải dữ liệu mã ${symbol}:`, error.message);
        return null;
    }
}

function calculateCAGR(data) {
    if (!data || !data.c || data.c.length < 2) return null;

    const closes = data.c; // Mảng giá đóng cửa
    const times = data.t;  // Mảng timestamp tương ứng

    const startPrice = closes[0];
    const endPrice = closes[closes.length - 1];

    const startTime = times[0];
    const endTime = times[times.length - 1];

    // Tính chính xác số năm dựa trên khoảng thời gian thực tế thu về từ dữ liệu
    const years = (endTime - startTime) / (365 * 24 * 60 * 60);
    if (years <= 0) return null;

    // Công thức CAGR = (End / Start)^(1 / years) - 1
    const cagr = Math.pow(endPrice / startPrice, 1 / years) - 1;
    
    return {
        startPrice,
        endPrice,
        years: years.toFixed(2),
        cagrPercent: (cagr * 100).toFixed(2)
    };
}

async function run() {
    console.log("Đang tính toán lãi kép trung bình (CAGR) 10 năm cho rổ VN30...\n");
    console.log("Mã CK\t | Giá Đầu Kỳ\t | Giá Cuối Kỳ\t | Số Năm\t | CAGR Trung Bình/Năm");
    console.log("-".repeat(75));

    for (const symbol of vn30Symbols) {
        const data = await fetchStockData(symbol);
        if (data && data.c && data.c.length > 0) {
            const result = calculateCAGR(data);
            if (result) {
                console.log(`${symbol.padEnd(8)} | ${result.startPrice.toString().padEnd(12)} | ${result.endPrice.toString().padEnd(12)} | ${result.years.toString().padEnd(7)} | ${result.cagrPercent}%`);
            } else {
                console.log(`${symbol.padEnd(8)} | Không đủ dữ liệu tính toán`);
            }
        } else {
            console.log(`${symbol.padEnd(8)} | Không gọi được API hoặc mã không tồn tại`);
        }
        
        // Nghỉ 200ms giữa các request để tránh nghẽn server API
        await new Promise(resolve => setTimeout(resolve, 200));
    }
}

run();