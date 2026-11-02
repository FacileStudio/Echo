export function gridRows(count: number) {
	return Math.max(1, Math.ceil(Math.sqrt(count)));
}

export function gridCols(count: number, rows: number) {
	return Math.max(1, Math.ceil(count / rows));
}

export function tileWidth(rows: number) {
	return `min(100%, calc((100cqh - ${rows - 1} * 0.25rem) / ${rows} * 16 / 9))`;
}

export function gridColumn(i: number, count: number, rows: number, cols: number) {
	const lastRowStart = (rows - 1) * cols;
	const remainder = count - lastRowStart;
	const offset = i === lastRowStart && remainder < cols ? cols - remainder + 1 : 'auto';

	return `${offset} / span 2`;
}
